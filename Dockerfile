FROM node:22-alpine3.20 as node

FROM golang:1.23-alpine3.20

COPY --from=node /usr/lib /usr/lib
COPY --from=node /usr/local/lib /usr/local/lib
COPY --from=node /usr/local/include /usr/local/include
COPY --from=node /usr/local/bin /usr/local/bin

WORKDIR /app

COPY ./ ./

RUN npm install
RUN npm run build
RUN go build -o /bin/gopxl-docs .

ENTRYPOINT ["./entrypoint.sh"]