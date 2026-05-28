# Usa la imagen oficial de PostgreSQL 18
FROM postgres:18

# Variables de entorno básicas para desarrollo con usuario y contraseña root
ENV POSTGRES_USER=root
ENV POSTGRES_PASSWORD=root

# Base de datos inicial
ENV POSTGRES_DB=auction_db

# Postgres escucha en 5432 DENTRO del contenedor.
# El puerto del host (5435) se define en docker-compose (5435:5432).
EXPOSE 5432
