FROM maven:3.9-eclipse-temurin-17-alpine AS build
WORKDIR /src
COPY pom.xml .
COPY src src
RUN mvn -q -DskipTests package

FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY --from=build /src/target/*.jar /app/app.jar
ENV PORT=8080
EXPOSE 8080
CMD ["sh", "-c", "java -jar /app/app.jar --server.address=0.0.0.0 --server.port=${PORT}"]
