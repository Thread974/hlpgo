FROM debian:buster

RUN apt update
RUN apt -y upgrade
RUN apt -y install golang-go

RUN apt -y install git
RUN go get github.com/Thread974/hlpgo
RUN go run github.com/Thread974/hlpgo --generate
