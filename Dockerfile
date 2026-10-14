FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /findedmevents ./cmd/findedmevents

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /findedmevents /findedmevents
EXPOSE 8080
ENTRYPOINT ["/findedmevents", "-http", ":8080"]
