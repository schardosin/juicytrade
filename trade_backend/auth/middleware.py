"""
Authentication middleware for juicytrade.
"""
import logging
from datetime import datetime, timedelta, timezone
from typing import Optional, Callable, Dict, Any
import jwt
from fastapi import Request, Response, HTTPException, status
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from starlette.middleware.base import BaseHTTPMiddleware

from .config import auth_config, AuthMethod
from .models import User, TokenData

logger = logging.getLogger(__name__)

# Security scheme for JWT tokens
security = HTTPBearer(auto_error=False)


class AuthenticationMiddleware(BaseHTTPMiddleware):
    """
    Authentication middleware that supports multiple authentication methods.
    """
    
    def __init__(self, app, exempt_paths: Optional[list] = None):
        super().__init__(app)
        self.exempt_paths = exempt_paths or [
            "/",
            "/docs",
            "/redoc",
            "/openapi.json",
            "/health",
            "/auth/login",
            "/auth/logout",
            "/auth/oauth/authorize",
            "/auth/oauth/callback",
            "/auth/config",
            "/auth/status",
            "/ws"
        ]
        
        # Validate configuration on startup
        is_valid, errors = auth_config.validate_config()
        if not is_valid:
            logger.error(f"Authentication configuration errors: {errors}")
            if auth_config.is_enabled():
                raise ValueError(f"Invalid authentication configuration: {errors}")
    
    async def dispatch(self, request: Request, call_next: Callable) -> Response:
        """Process request through authentication middleware."""
        
        # Skip authentication if disabled
        if not auth_config.is_enabled():
            return await call_next(request)
        
        # Check if path is exempt from authentication
        if self._is_exempt_path(request.url.path):
            return await call_next(request)
        
        # Special handling for WebSocket upgrade requests
        # Let them pass through to the WebSocket endpoint where proper WebSocket auth is handled
        if (request.url.path == "/ws" and 
            request.headers.get("upgrade", "").lower() == "websocket"):
            return await call_next(request)
        
        # Perform authentication based on configured method
        try:
            user = await self._authenticate_request(request)
            if user:
                # Add user to request state
                request.state.user = user
                request.state.authenticated = True
            else:
                # Authentication failed
                return self._create_auth_error_response(request)
        
        except HTTPException as e:
            return self._create_auth_error_response(request, e.detail, e.status_code)
        except Exception as e:
            logger.error(f"Authentication error: {e}")
            return self._create_auth_error_response(request, "Authentication failed")
        
        return await call_next(request)
    
    def _is_exempt_path(self, path: str) -> bool:
        """Check if path is exempt from authentication."""
        # Fix: Only match exact path or path with trailing slash, not prefix
        return any(path == exempt or path == (exempt + "/") for exempt in self.exempt_paths)
    
    async def _authenticate_request(self, request: Request) -> Optional[User]:
        """Authenticate request based on configured method."""
        
        if auth_config.method == AuthMethod.SIMPLE:
            return await self._authenticate_simple(request)
        
        elif auth_config.method == AuthMethod.TOKEN:
            return await self._authenticate_token(request)
        
        elif auth_config.method == AuthMethod.HEADER:
            return await self._authenticate_header(request)
        
        elif auth_config.method == AuthMethod.OAUTH:
            return await self._authenticate_oauth(request)
        
        return None
    
    async def _authenticate_simple(self, request: Request) -> Optional[User]:
        """Authenticate using simple username/password or session."""
        
        # Check for existing session first
        session_token = request.cookies.get(auth_config.session_cookie_name)
        if session_token:
            user = await self._validate_session_token(session_token)
            if user:
                return user
        
        # For API requests, require Authorization header
        auth_header = request.headers.get("Authorization")
        if auth_header and auth_header.startswith("Basic "):
            import base64
            try:
                encoded_credentials = auth_header.split(" ", 1)[1]
                credentials = base64.b64decode(encoded_credentials).decode("utf-8")
                
                # Safe credential parsing - handle malformed credentials
                if ':' not in credentials:
                    logger.warning(f"Basic auth credentials missing colon separator: '{credentials}'")
                    return None
                
                credential_parts = credentials.split(":", 1)
                if len(credential_parts) != 2:
                    logger.warning(f"Basic auth credentials malformed - got {len(credential_parts)} parts: {credential_parts}")
                    return None
                
                username, password = credential_parts
                
                if (username == auth_config.simple_username and 
                    password == auth_config.simple_password):
                    return User(
                        username=username,
                        email=None,
                        full_name=username,
                        last_login=datetime.now(timezone.utc)
                    )
            except Exception as e:
                logger.warning(f"Invalid Basic auth credentials: {e}")
        
        return None
    
    async def _authenticate_token(self, request: Request) -> Optional[User]:
        """Authenticate using JWT token."""
        
        # Try Authorization header first
        auth_header = request.headers.get("Authorization")
        token = None
        
        if auth_header and auth_header.startswith("Bearer "):
            token = auth_header.split(" ", 1)[1]
        else:
            # Try cookie
            token = request.cookies.get(auth_config.session_cookie_name)
        
        if not token:
            return None
        
        try:
            payload = jwt.decode(
                token,
                auth_config.jwt_secret_key,
                algorithms=[auth_config.jwt_algorithm]
            )
            
            username = payload.get("sub")
            expires_at = datetime.fromtimestamp(payload.get("exp", 0), tz=timezone.utc)
            
            if not username or expires_at < datetime.now(timezone.utc):
                return None
            
            return User(
                username=username,
                email=payload.get("email"),
                full_name=payload.get("name"),
                last_login=datetime.now(timezone.utc),
                metadata=payload.get("metadata")
            )
            
        except jwt.InvalidTokenError as e:
            logger.warning(f"Invalid JWT token: {e}")
            return None
    
    async def _authenticate_header(self, request: Request) -> Optional[User]:
        """Authenticate using header-based authentication (for reverse proxy)."""
        
        username = request.headers.get(auth_config.header_name)
        if not username:
            return None
        
        # Get additional user info from headers if available
        email = request.headers.get("X-Remote-Email")
        full_name = request.headers.get("X-Remote-Name")
        
        return User(
            username=username,
            email=email,
            full_name=full_name or username,
            last_login=datetime.now(timezone.utc)
        )
    
    async def _authenticate_oauth(self, request: Request) -> Optional[User]:
        """Authenticate using OAuth (check for session after OAuth flow)."""
        
        # OAuth authentication happens through login endpoints
        # Here we just check for existing session
        session_token = request.cookies.get(auth_config.session_cookie_name)
        if session_token:
            return await self._validate_session_token(session_token)
        
        return None
    
    async def _validate_session_token(self, token: str) -> Optional[User]:
        """Validate session token (JWT-based sessions)."""
        try:
            payload = jwt.decode(
                token,
                auth_config.jwt_secret_key,
                algorithms=[auth_config.jwt_algorithm]
            )
            
            username = payload.get("sub")
            expires_at = datetime.fromtimestamp(payload.get("exp", 0), tz=timezone.utc)
            
            if not username or expires_at < datetime.now(timezone.utc):
                return None
            
            return User(
                username=username,
                email=payload.get("email"),
                full_name=payload.get("name"),
                last_login=datetime.now(timezone.utc),
                metadata=payload.get("metadata")
            )
            
        except jwt.InvalidTokenError:
            return None
    
    def _create_auth_error_response(self, request: Request, message: str = "Authentication required", status_code: int = 401) -> Response:
        """Create authentication error response."""
        
        # For API requests, return JSON
        if request.url.path.startswith("/api/") or request.headers.get("Accept", "").startswith("application/json"):
            from fastapi.responses import JSONResponse
            return JSONResponse(
                status_code=status_code,
                content={
                    "success": False,
                    "message": message,
                    "error": "authentication_required"
                }
            )
        
        # For web requests, redirect to login
        from fastapi.responses import RedirectResponse
        login_url = f"/auth/login?next={request.url.path}"
        return RedirectResponse(url=login_url, status_code=302)


def create_access_token(user: User, expires_delta: Optional[timedelta] = None) -> str:
    """Create JWT access token for user."""
    
    if expires_delta:
        expire = datetime.now(timezone.utc) + expires_delta
    else:
        expire = datetime.now(timezone.utc) + timedelta(minutes=auth_config.jwt_expire_minutes)
    
    payload = {
        "sub": user.username,
        "exp": expire,
        "iat": datetime.now(timezone.utc),
        "email": user.email,
        "name": user.full_name,
        "metadata": user.metadata
    }
    
    return jwt.encode(payload, auth_config.jwt_secret_key, algorithm=auth_config.jwt_algorithm)


def get_current_user(request: Request) -> Optional[User]:
    """Get current authenticated user from request."""
    return getattr(request.state, "user", None)


def require_auth(request: Request) -> User:
    """Require authentication and return current user."""
    user = get_current_user(request)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Authentication required"
        )
    return user
