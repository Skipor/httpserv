

all:
	go get | true
	go build ./httpserv.go

clean:
	rm -f httpserv
