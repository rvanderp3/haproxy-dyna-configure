FROM golang:1.20

WORKDIR /usr/src/app


COPY . .
RUN go mod tidy && go mod vendor

COPY run.sh .

RUN ./hack/build.sh
CMD ./run.sh

