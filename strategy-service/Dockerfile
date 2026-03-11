FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY src/ /app/src/

# Create data directory
RUN mkdir -p /app/data/parquet

# Expose port
EXPOSE 8009

# Run the service
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8009"]
