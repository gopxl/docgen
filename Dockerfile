# Container image that runs your code
FROM golang:1.23-alpine3.20

WORKDIR /app

COPY ./ ./

RUN go build -o /bin/gopxl-docs .

# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["gopxl-docs"]
