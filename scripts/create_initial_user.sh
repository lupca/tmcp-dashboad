#!/bin/bash

# Default values
EMAIL="${1:-admin@tmcp.com}"
PASSWORD="${2:-1234567890}"

echo "Creating initial superuser: $EMAIL..."
go run . superuser upsert "$EMAIL" "$PASSWORD"
