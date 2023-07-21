# Build
FROM --platform=linux/amd64 golang:1.19 as build

WORKDIR /app
COPY . ./
RUN go build -o /keess

# Run
FROM --platform=linux/amd64 ubuntu

WORKDIR /
COPY --from=build /keess /keess

RUN echo "#!/bin/bash \nexec ./keess run --sourceContext=$SOURCE_CONTEXT --destinationContexts=$DESTINATION_CONTEXTS \n" > ./entrypoint.sh

RUN chmod +x ./entrypoint.sh
ENTRYPOINT ["./entrypoint.sh"]