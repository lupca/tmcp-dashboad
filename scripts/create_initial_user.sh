#!/bin/bash

# Default values
EMAIL="${1:-admin@tmcp.com}"
PASSWORD="${2:-1234567890}"

echo "Creating initial superuser: $EMAIL..."
go run main.go superuser upsert "$EMAIL" "$PASSWORD"
