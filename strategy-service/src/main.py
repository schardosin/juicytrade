import logging
import sys
from pathlib import Path
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

# Add parent directory to path so 'src' packages can be imported
_parent_dir = Path(__file__).parent.parent.parent
if str(_parent_dir) not in sys.path:
    sys.path.insert(0, str(_parent_dir))

from src.config import settings
from src.api.endpoints import router as strategies_router
from src.api.data_import_endpoints import router as data_import_router
from src.persistence.database import strategy_db_manager

# Configure logging
logging.basicConfig(
    level=settings.log_level,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("strategy-service")

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager."""
    # Startup
    logger.info("🚀 Starting Strategy Service...")
    
    # Initialize database
    logger.info("🔄 Initializing Strategy Database...")
    db_success = strategy_db_manager.initialize()
    if db_success:
        logger.info("✅ Strategy Database initialized")
    else:
        logger.error("❌ Strategy Database initialization failed")
    
    yield
    
    # Shutdown
    logger.info("🛑 Shutting down Strategy Service...")

# Create FastAPI app
app = FastAPI(
    title="Strategy Service API",
    description="Strategy backtesting and execution microservice",
    version="1.0.0",
    lifespan=lifespan
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:3001", "http://localhost:8008"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Register routers
app.include_router(strategies_router, prefix="/api/strategies", tags=["strategies"])
app.include_router(data_import_router, prefix="/api", tags=["data-import"])

@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {
        "status": "healthy",
        "service": settings.service_name
    }

@app.get("/")
async def root():
    """Root endpoint."""
    return {
        "service": settings.service_name,
        "version": "1.0.0",
        "docs": "/docs"
    }

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "src.main:app",
        host=settings.host,
        port=settings.port,
        reload=True
    )
