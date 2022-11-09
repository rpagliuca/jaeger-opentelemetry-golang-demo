FROM golang
WORKDIR  /app
COPY . /app
RUN go build -o app .
ENTRYPOINT ./app
