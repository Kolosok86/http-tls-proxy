# Start by building the application.
FROM golang:1.20 as build

WORKDIR /usr/src/proxy
COPY . .

RUN CGO_ENABLED=0 go build -o proxy ./cmd/

# Now copy it into our base image.
FROM gcr.io/distroless/static-debian11:nonroot
COPY --from=build /usr/src/proxy/proxy /usr/bin/proxy

EXPOSE 3128

CMD [ "/usr/bin/proxy" ]
