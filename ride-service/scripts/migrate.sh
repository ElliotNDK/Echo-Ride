#!/bin/bash

set -e

source .env

## Check if the database is ready
#until pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER"; do
#  echo "Waiting for the database to be ready..."
#  sleep 2
#done

export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="postgres://${RIDE_DB_USER}:${RIDE_DB_PASSWORD}@${RIDE_DB_HOST}:${RIDE_DB_PORT}/${RIDE_DB_NAME}?sslmode=disable"
export GOOSE_MIGRATION_DIR=./sql/migrations

goose "$@"