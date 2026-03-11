# Strategy Service

A high-performance Python microservice for trading strategy backtesting and execution.

## Features
- **Strategy Framework**: Declarative API for writing trading strategies in Python
- **Backtesting Engine**: Fast event-driven backtesting with historical data
- **Data Aggregation**: Efficient OHLCV aggregation using DuckDB
- **REST API**: Comprehensive API for strategy management and execution

## Project Structure
```
strategy-service/
├── src/
│   ├── api/            # REST API endpoints
│   ├── core/           # Core strategy framework
│   ├── execution/      # Execution engine & order management
│   ├── backtest/       # Backtesting engine
│   ├── data/           # Data providers & aggregation
│   ├── persistence/    # Database & file storage
│   ├── models/         # Domain models
│   └── examples/       # Example strategies
```

## Getting Started

### Prerequisites
- Python 3.11+
- Docker (optional)

### Local Development

1. Create a virtual environment:
   ```bash
   python -m venv venv
   source venv/bin/activate
   ```

2. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```

3. Run the service:
   ```bash
   python -m src.main
   ```

The API will be available at `http://localhost:8009`.

### Docker

Build and run using Docker Compose:
```bash
docker-compose up --build
```

## API Documentation
Once running, visit `http://localhost:8009/docs` for the interactive API documentation.
