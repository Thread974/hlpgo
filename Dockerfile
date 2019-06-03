FROM debian:buster

RUN apt update
RUN apt -y upgrade
RUN apt -y install golang-go git

RUN cd /root && go get github.com/Thread974/hlpgo
RUN cd /root && /root/go/bin/hlpgo --test
RUN cd /root && go build github.com/Thread974/hlpgo
RUN cd /root && /root/go/bin/hlpgo --help && echo "hlp go is running properly"
