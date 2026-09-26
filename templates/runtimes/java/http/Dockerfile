# Generic Java HTTP function (RFC-0001 §6): bind 0.0.0.0:$PORT, logs to stdout.
FROM eclipse-temurin:17-jdk-alpine AS build
WORKDIR /src
COPY Handler.java .
RUN javac Handler.java

FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY --from=build /src/*.class /app/
ENV PORT=8080
EXPOSE 8080
CMD ["java", "Handler"]
