# java-spring-jpa stack pack (RFC-0001 §6 / Phase 7):
# Multi-stage Maven (or Gradle wrapper) build; listen on 0.0.0.0:$PORT / SERVER_PORT.
FROM eclipse-temurin:17-jdk-alpine AS build
WORKDIR /src
COPY . .
RUN set -eux; \
    mkdir -p /out; \
    if [ -f pom.xml ]; then \
      apk add --no-cache maven; \
      mvn -q -B -DskipTests package; \
      cp target/*.jar /out/app.jar; \
    elif [ -f gradlew ]; then \
      chmod +x gradlew; \
      ./gradlew bootJar -x test --no-daemon; \
      cp build/libs/*.jar /out/app.jar; \
    else \
      echo "missing pom.xml or gradlew" >&2; exit 1; \
    fi

FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
RUN adduser -S -u 10001 app
COPY --from=build /out/app.jar /app/app.jar
ENV PORT=8080 \
    SERVER_PORT=8080
EXPOSE 8080
USER 10001
CMD ["sh", "-c", "java -jar /app/app.jar --server.address=0.0.0.0 --server.port=${PORT}"]
