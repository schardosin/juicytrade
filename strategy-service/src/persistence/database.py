"""
Database manager for strategy persistence.
Follows the same patterns as existing system components (PathManager, ProviderCredentialStore).
"""

import logging
from pathlib import Path
from typing import Optional
from sqlalchemy import create_engine, event, text
from sqlalchemy.orm import sessionmaker, Session
from sqlalchemy.engine import Engine
from contextlib import contextmanager
try:
    from src.path_manager import path_manager
except ImportError:
    # Fallback for direct execution
    import sys
    from pathlib import Path
    sys.path.append(str(Path(__file__).parent.parent))
    from src.path_manager import path_manager
from src.persistence.models import Base

logger = logging.getLogger(__name__)

class StrategyDatabaseManager:
    """
    Database manager for strategy persistence.
    Follows PathManager pattern for automatic container/development mode detection.
    """
    
    def __init__(self, db_filename: str = "strategies.db"):
        self.db_filename = db_filename
        self.db_path = None
        self.engine = None
        self.SessionLocal = None
        self._initialized = False
    
    def initialize(self) -> bool:
        """
        Initialize database connection using PathManager pattern.
        Returns True if successful, False otherwise.
        """
        try:
            # Use PathManager to get the correct database path
            self.db_path = path_manager.get_config_file_path(self.db_filename)
            
            # Create SQLite engine with proper configuration
            database_url = f"sqlite:///{self.db_path}"
            self.engine = create_engine(
                database_url,
                echo=False,  # Set to True for SQL debugging
                pool_pre_ping=True,
                connect_args={
                    "check_same_thread": False,  # Allow multiple threads
                    "timeout": 30  # 30 second timeout
                }
            )
            
            # Enable foreign key constraints for SQLite
            @event.listens_for(Engine, "connect")
            def set_sqlite_pragma(dbapi_connection, connection_record):
                if 'sqlite' in str(dbapi_connection):
                    cursor = dbapi_connection.cursor()
                    cursor.execute("PRAGMA foreign_keys=ON")
                    cursor.execute("PRAGMA journal_mode=WAL")  # Better concurrency
                    cursor.execute("PRAGMA synchronous=NORMAL")  # Better performance
                    cursor.close()
            
            # Create session factory
            self.SessionLocal = sessionmaker(
                autocommit=False,
                autoflush=False,
                bind=self.engine
            )
            
            # Create all tables
            Base.metadata.create_all(bind=self.engine)
            
            self._initialized = True
            
            # Log initialization success (following existing logging patterns)
            mode = "container" if path_manager.is_container_mode() else "development"
            logger.info(f"📁 Strategy database initialized in {mode} mode: {self.db_path}")
            
            return True
            
        except Exception as e:
            logger.error(f"❌ Failed to initialize strategy database: {e}")
            return False
    
    @contextmanager
    def get_session(self) -> Session:
        """
        Get database session with automatic cleanup.
        Follows the same pattern as existing database operations.
        """
        if not self._initialized:
            raise RuntimeError("Database not initialized. Call initialize() first.")
        
        session = self.SessionLocal()
        try:
            yield session
            session.commit()
        except Exception as e:
            session.rollback()
            logger.error(f"❌ Database session error: {e}")
            raise
        finally:
            session.close()
    
    def get_session_direct(self) -> Session:
        """
        Get database session for manual management.
        Use with caution - remember to close the session.
        """
        if not self._initialized:
            raise RuntimeError("Database not initialized. Call initialize() first.")
        
        return self.SessionLocal()
    
    def health_check(self) -> dict:
        """
        Perform database health check.
        Returns status information similar to PathManager.get_status().
        """
        try:
            if not self._initialized:
                return {
                    "status": "not_initialized",
                    "database_path": None,
                    "connection": False,
                    "tables_exist": False
                }
            
            # Test database connection
            with self.get_session() as session:
                # Simple query to test connection
                result = session.execute(text("SELECT 1")).fetchone()
                connection_ok = result is not None
            
            # Check if tables exist
            from sqlalchemy import inspect
            inspector = inspect(self.engine)
            tables_exist = inspector.has_table("strategies")
            
            return {
                "status": "healthy" if connection_ok and tables_exist else "unhealthy",
                "database_path": str(self.db_path),
                "connection": connection_ok,
                "tables_exist": tables_exist,
                "mode": "container" if path_manager.is_container_mode() else "development",
                "file_exists": self.db_path.exists() if self.db_path else False,
                "file_size": self.db_path.stat().st_size if self.db_path and self.db_path.exists() else 0
            }
            
        except Exception as e:
            logger.error(f"❌ Database health check failed: {e}")
            return {
                "status": "error",
                "error": str(e),
                "database_path": str(self.db_path) if self.db_path else None,
                "connection": False,
                "tables_exist": False
            }
    
    def backup_database(self, backup_filename: Optional[str] = None) -> bool:
        """
        Create a backup of the database.
        Returns True if successful, False otherwise.
        """
        try:
            if not self._initialized or not self.db_path.exists():
                logger.warning("⚠️ Cannot backup: database not initialized or doesn't exist")
                return False
            
            # Generate backup filename if not provided
            if not backup_filename:
                from datetime import datetime
                timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
                backup_filename = f"strategies_backup_{timestamp}.db"
            
            # Get backup path using PathManager
            backup_path = path_manager.get_config_file_path(backup_filename)
            
            # Copy database file
            import shutil
            shutil.copy2(self.db_path, backup_path)
            
            logger.info(f"💾 Database backup created: {backup_path}")
            return True
            
        except Exception as e:
            logger.error(f"❌ Database backup failed: {e}")
            return False
    
    def restore_database(self, backup_filename: str) -> bool:
        """
        Restore database from backup.
        Returns True if successful, False otherwise.
        """
        try:
            # Get backup path using PathManager
            backup_path = path_manager.get_config_file_path(backup_filename)
            
            if not backup_path.exists():
                logger.error(f"❌ Backup file not found: {backup_path}")
                return False
            
            # Close existing connections
            if self.engine:
                self.engine.dispose()
            
            # Copy backup to main database
            import shutil
            shutil.copy2(backup_path, self.db_path)
            
            # Reinitialize database
            self.initialize()
            
            logger.info(f"🔄 Database restored from backup: {backup_filename}")
            return True
            
        except Exception as e:
            logger.error(f"❌ Database restore failed: {e}")
            return False
    
    def get_database_stats(self) -> dict:
        """
        Get database statistics for monitoring.
        """
        try:
            if not self._initialized:
                return {"error": "Database not initialized"}
            
            with self.get_session() as session:
                # Count records in each table
                from src.persistence.models import Strategy, StrategyExecution, StrategyTrade, StrategyPerformance
                
                strategy_count = session.query(Strategy).filter(Strategy.is_active == True).count()
                execution_count = session.query(StrategyExecution).count()
                trade_count = session.query(StrategyTrade).count()
                performance_count = session.query(StrategyPerformance).count()
                
                # Get file size
                file_size = self.db_path.stat().st_size if self.db_path.exists() else 0
                
                return {
                    "strategies": strategy_count,
                    "executions": execution_count,
                    "trades": trade_count,
                    "performance_records": performance_count,
                    "database_size_bytes": file_size,
                    "database_size_mb": round(file_size / (1024 * 1024), 2)
                }
                
        except Exception as e:
            logger.error(f"❌ Failed to get database stats: {e}")
            return {"error": str(e)}

# Global instance (following the same pattern as path_manager)
strategy_db_manager = StrategyDatabaseManager()
