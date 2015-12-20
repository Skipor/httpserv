package main

import (
	. "./fd"
	"log"
	sys "golang.org/x/sys/unix"
	"net"
	"strings"
	"bytes"
	"io"
	"os"
	"errors"
	"strconv"
)

const (
	httpVersion = "HTTP/1.1"
	notFoundResponse = "HTTP/1.1 404 Not Found"
	CRLF = "\r\n"
)

var (
	resDir string = "./res"
	statusMap = map[int]string{
		200: "200 OK",
		404: "404 Not Found",
		500: "500 Internal Server Error",
	}

	resMap = map[string]string {
		"/": "/index.html",
	}
)



func getStatusByCode(code int) string {
	if ret := statusMap[code]; ret != "" {
		return httpVersion + " " + ret + CRLF
	} else {
		log.Printf("Error: unsuported code request %d:", code)
		return httpVersion + " " + statusMap[500] + CRLF
	}
}

func getResourceResponce(uri string) (headers []byte, bodyReader io.ReadCloser) {

	var headersBuf bytes.Buffer
	if alias := resMap[uri]; alias != "" {
		uri = alias
	}

	fileName := resDir + uri
	if stat, err := os.Stat(fileName); err == nil {
		if !stat.IsDir() {
			if file, err := os.Open(fileName); err == nil {
				bodyReader = file
				headersBuf.WriteString(getStatusByCode(200))
				headersBuf.WriteString("Content-Length: ")
				fileSizeStr := strconv.FormatInt(stat.Size(), 10)
				headersBuf.WriteString(fileSizeStr)
				headersBuf.WriteString(CRLF)
				headersBuf.WriteString(CRLF)
			} else {
				log.Printf("Resorce open error: %s", err)
				headersBuf.WriteString(getStatusByCode(404))
			}
		} else {
			log.Printf("Resouce is dir: %s", err)
			headersBuf.WriteString(getStatusByCode(404))
		}
	} else {
		log.Printf("Get resouce error: %s", err)
		headersBuf.WriteString(getStatusByCode(404))
	}
	headers = headersBuf.Bytes()
	return
}

func getReqLine(sock Fd, addr sys.Sockaddr) (reqLine string, resErr error) {
	request := make([]byte, 0)
	const BuffSize = 4096
	buff := make([]byte, BuffSize)
	for {
		if n, err := sock.Read(buff); err == nil {
			if n == 0 {
				break
			}
			request = append(request, buff[:n]...)
			if bytes.Contains(request, []byte("\r\n")) { // have
				break
			}
		} else {
			if err == io.EOF {
				break
			} else {
				resErr = err
				return
			}
		}
	}
	reqLine = strings.SplitN(string(request), "\r\n", 2)[0]
	return
}


func handleClient(sock Fd, addr sys.Sockaddr) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in client handle: %s", r)
		}
	}()

	clientPort := addr.(*sys.SockaddrInet4).Port
	defer func() {
		sock.Close()
		log.Printf("Close client on port: %d", clientPort)
	}()


	reqLine, err := getReqLine(sock, addr)
	if err != nil {
		log.Printf("Error client request read on port: %d", clientPort)
	}
	log.Printf("ReqLine: %s", reqLine)



	reqLineTokens := strings.Fields(reqLine)
	if len(reqLineTokens) < 2 {
		log.Printf("Mailformed request line '%s' on client port: %d", reqLine, clientPort)
		return
	}

	method := reqLineTokens[0]
	uri := reqLineTokens[1]
	log.Printf("Method: %s, URI: %s", method, uri)


	var respReader io.Reader
	switch method {
	case "GET", "HEAD":
		headers, bodyReader := getResourceResponce(uri)
		if bodyReader != nil {
			defer bodyReader.Close()
		}
		respReader = io.MultiReader(
			bytes.NewBuffer(headers),
		)
		if method == "GET" && bodyReader != nil {
			respReader = io.MultiReader(
				respReader,
				bodyReader,
			)
		}

	default:
		respReader = io.MultiReader(
			bytes.NewBufferString(getStatusByCode(404) + CRLF),
		)

	}

	if n, err := io.Copy(sock, respReader); err == nil {
		log.Printf("Send resp of %d bytes to client on port %d", n, clientPort)
	} else {
		log.Printf("Senr resp to client on port %d failed: ", clientPort, err)
	}
}

func getPort() (port int, err error) {
	cmdArgs := os.Args[1:]
	if len(cmdArgs) != 1 {
		err = errors.New("Please pass exacly one argument: port number")
		return
	}
	port, err = strconv.Atoi(cmdArgs[0])
	return
}


func checkResDir() (err error) {
	//check res dir
	if file, err := os.Open(resDir); err == nil {
		defer file.Close()
		if stat, err := file.Stat(); err == nil {
			if stat.IsDir() == false {
				err = errors.New("Resouse dir is not dirrectory")
			}
		} else {
			err = errors.New("Cannot resource dir info")
		}
	} else {
		err = errors.New("Cannot open resource dir ")
	}
	return
}
func main() {

	port, err := getPort()
	if err != nil {
		log.Fatal(err)
	}

	if err := checkResDir(); err != nil {
		log.Fatal(err)
	}



	servSock, err := sys.Socket(sys.AF_INET, sys.SOCK_STREAM, 0)
	if err != nil {
		log.Fatalf("Cannot get socket: %s", err)
	}
	defer sys.Close(servSock)

	if err := sys.SetsockoptInt(servSock, sys.SOL_SOCKET, sys.SO_REUSEADDR, 1); err != nil {
		log.Fatalf("Cannot socket opt: %s", err)
	}


	servSockAddr := sys.SockaddrInet4{Port: port}
	copy(servSockAddr.Addr[:], net.ParseIP("0.0.0.0").To4())

	if err := sys.Bind(servSock, &servSockAddr); err != nil {
		log.Fatalf("Cannot bind socket to addr %d: %s", port, err)
	}

	if err := sys.Listen(servSock, 64); err != nil {
		log.Fatalf("Cannot listen socket on addr %d: %s", port, err)
	}
	log.Printf("httpserv is listening port: %d", port)

	for {
		if clientSock, clientSockAddr, err := sys.Accept(servSock); err == nil {
			inetSockAddr := clientSockAddr.(*sys.SockaddrInet4)
			log.Printf("Accepted client on port: %d", inetSockAddr.Port)
			go handleClient(Fd(clientSock), clientSockAddr)
		} else {
			log.Printf("Unanable accept connection: %s", err)
		}
	}

}