#!/bin/bash

# Script to pull and run the email-validator Docker image

# Default values
DOCKER_USERNAME="umutert"
PORT=8080
TAG="latest"
REDIS_ENABLED=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --username|-u)
      DOCKER_USERNAME="$2"
      shift
      shift
      ;;
    --port|-p)
      PORT="$2"
      shift
      shift
      ;;
    --tag|-t)
      TAG="$2"
      shift
      shift
      ;;
    --redis|-r)
      REDIS_ENABLED=true
      shift
      ;;
    --help|-h)
      echo "Usage: $0 [OPTIONS]"
      echo "Pull and run the email-validator Docker image."
      echo ""
      echo "Options:"
      echo "  -u, --username USERNAME   Docker Hub username (default: USERNAME)"
      echo "  -p, --port PORT           Port to expose (default: 8080)"
      echo "  -t, --tag TAG             Docker image tag (default: latest)"
      echo "  -r, --redis               Enable Redis connection"
      echo "  -h, --help                Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Run '$0 --help' for usage information."
      exit 1
      ;;
  esac
done

# Pull the Docker image
echo "Pulling Docker image ${DOCKER_USERNAME}/emailvalidator:${TAG}..."
docker pull "${DOCKER_USERNAME}/emailvalidator:${TAG}"

# Prepare environment variables
ENV_VARS="-e PORT=${PORT}"

if [ "$REDIS_ENABLED" = true ]; then
  echo "Starting Redis container..."
  docker run -d --name email-validator-redis redis:alpine
  
  # Get the Redis container IP
  REDIS_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' email-validator-redis)
  ENV_VARS="${ENV_VARS} -e REDIS_URL=redis://${REDIS_IP}:6379"
  echo "Redis running at ${REDIS_IP}:6379"
fi

# Run the Docker container
echo "Starting email-validator on port ${PORT}..."
docker run -d --name email-validator ${ENV_VARS} -p "${PORT}:${PORT}" "${DOCKER_USERNAME}/emailvalidator:${TAG}"

echo "Email validator service is now running!"
echo "Access the service at: http://localhost:${PORT}"
echo ""
echo "To stop the service: docker stop email-validator"
if [ "$REDIS_ENABLED" = true ]; then
  echo "To stop Redis: docker stop email-validator-redis"
fi 