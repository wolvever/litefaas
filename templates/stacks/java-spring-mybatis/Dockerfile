# java-spring-mybatis stack pack (RFC-0001 §6 / Phase 7):
# Maven multi-stage build; listen on 0.0.0.0:$PORT.
FROM maven:3.9-eclipse-temurin-17-alpine AS build
WORKDIR /src
COPY pom.xml .
COPY src src
RUN mvn -q -B -DskipTests package

FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
RUN adduser -S -u 10001 app
COPY --from=build /src/target/*.jar /app/app.jar
ENV PORT=8080
EXPOSE 8080
USER 10001
CMD ["sh", "-c", "java -jar /app/app.jar --server.address=0.0.0.0 --server.port=${PORT}"]
