"""
Authentication endpoints for juicytrade.
"""
import logging
import secrets
import urllib.parse
from datetime import datetime, timedelta, timezone
from typing import Optional, Dict, Any
from fastapi import APIRouter, Request, Response, HTTPException, status, Depends, Form
from fastapi.responses import RedirectResponse, JSONResponse, HTMLResponse
import httpx
import jwt

from .config import auth_config, AuthMethod
from .models import User, LoginRequest, LoginResponse, LogoutResponse, AuthStatus
from .middleware import create_access_token, get_current_user
from ..models import ApiResponse

logger = logging.getLogger(__name__)

# Create router for authentication endpoints
auth_router = APIRouter(prefix="/auth", tags=["authentication"])

# In-memory OAuth state storage (in production, use Redis or database)
oauth_states: Dict[str, Dict[str, Any]] = {}


@auth_router.get("/status", response_model=ApiResponse)
async def get_auth_status(request: Request):
    """Get current authentication status."""
    user = None
    authenticated = False
    
    # Manually check authentication since this endpoint is exempt from middleware
    if auth_config.is_enabled():
        # Check for session cookie
        session_token = request.cookies.get(auth_config.session_cookie_name)
        if session_token:
            try:
                payload = jwt.decode(
                    session_token,
                    auth_config.jwt_secret_key,
                    algorithms=[auth_config.jwt_algorithm]
                )
                
                username = payload.get("sub")
                expires_at = datetime.fromtimestamp(payload.get("exp", 0), tz=timezone.utc)
                
                if username and expires_at > datetime.now(timezone.utc):
                    user = User(
                        username=username,
                        email=payload.get("email"),
                        full_name=payload.get("name"),
                        last_login=datetime.now(timezone.utc),
                        metadata=payload.get("metadata")
                    )
                    authenticated = True
                    
            except jwt.InvalidTokenError as e:
                logger.warning(f"Invalid JWT token in status check: {e}")
                pass  # Invalid token, user remains None
            except Exception as e:
                logger.error(f"Error validating JWT token: {e}")
                pass
    
    return ApiResponse(
        success=True,
        data=AuthStatus(
            authenticated=authenticated,
            method=auth_config.method.value,
            user=user,
            expires_at=None  # Could be extracted from JWT if needed
        ).model_dump(),
        message="Authentication status retrieved"
    )


@auth_router.get("/config", response_model=ApiResponse)
async def get_auth_config():
    """Get public authentication configuration."""
    return ApiResponse(
        success=True,
        data={
            "method": auth_config.method.value,
            "enabled": auth_config.is_enabled(),
            "oauth_provider": auth_config.oauth_provider.value if auth_config.oauth_provider else None,
            "supports_methods": ["simple", "oauth", "token", "header", "disabled"],
            "session_cookie_name": auth_config.session_cookie_name
        },
        message="Authentication configuration retrieved"
    )


@auth_router.post("/login", response_model=ApiResponse)
async def login(
    login_data: LoginRequest,
    request: Request,
    response: Response
):
    """Login endpoint supporting JSON data."""
    
    if not auth_config.is_enabled():
        raise HTTPException(status_code=400, detail="Authentication is disabled")
    
    username = login_data.username
    password = login_data.password
    
    if not username or not password:
        raise HTTPException(status_code=400, detail="Username and password are required")
    
    # Authenticate based on configured method
    user = None
    
    if auth_config.method == AuthMethod.SIMPLE:
        if (username == auth_config.simple_username and 
            password == auth_config.simple_password):
            user = User(
                username=username,
                email=None,
                full_name=username,
                last_login=datetime.now(timezone.utc)
            )
    
    elif auth_config.method == AuthMethod.TOKEN:
        # For token method, validate against simple credentials if provided
        if (auth_config.simple_username and auth_config.simple_password and
            username == auth_config.simple_username and 
            password == auth_config.simple_password):
            user = User(
                username=username,
                email=None,
                full_name=username,
                last_login=datetime.now(timezone.utc)
            )
    
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid credentials"
        )
    
    # Create access token
    access_token = create_access_token(user)
    expires_in = auth_config.jwt_expire_minutes * 60
    
    # Set session cookie with proper domain configuration
    cookie_kwargs = {
        "key": auth_config.session_cookie_name,
        "value": access_token,
        "max_age": expires_in,
        "httponly": False,  # Allow JavaScript access for cross-port compatibility
        "secure": auth_config.secure_cookies,
        "samesite": "lax",
        "path": "/"
    }
    
    # Set domain if configured (for production), otherwise no domain restriction
    if auth_config.cookie_domain:
        cookie_kwargs["domain"] = auth_config.cookie_domain
    # No else clause - if no domain is configured, don't set domain restriction
    
    response.set_cookie(**cookie_kwargs)
    
    return ApiResponse(
        success=True,
        data=LoginResponse(
            success=True,
            message="Login successful",
            access_token=access_token,
            token_type="bearer",
            expires_in=expires_in,
            user=user
        ).model_dump(),
        message="Login successful"
    )


@auth_router.post("/logout", response_model=ApiResponse)
async def logout(request: Request, response: Response):
    """Logout endpoint."""
    
    # Clear all OAuth states to prevent issues with subsequent logins
    global oauth_states
    oauth_states.clear()
    
    # Clear session cookie with proper domain configuration
    cookie_kwargs = {
        "key": auth_config.session_cookie_name,
        "httponly": True,
        "secure": auth_config.secure_cookies,
        "samesite": "lax",
        "path": "/"
    }
    
    # Set domain if configured (for production), otherwise no domain restriction
    if auth_config.cookie_domain:
        cookie_kwargs["domain"] = auth_config.cookie_domain
    # No else clause - if no domain is configured, don't set domain restriction
    
    response.delete_cookie(**cookie_kwargs)
    
    return ApiResponse(
        success=True,
        data=LogoutResponse(
            success=True,
            message="Logout successful"
        ).model_dump(),
        message="Logout successful"
    )


@auth_router.get("/login")
async def login_page(request: Request, next: Optional[str] = None):
    """Login page for web interface."""
    
    if not auth_config.is_enabled():
        return HTMLResponse(
            content="<h1>Authentication is disabled</h1>",
            status_code=200
        )
    
    # Check if user is already authenticated
    user = get_current_user(request)
    if user:
        redirect_url = next or "/"
        return RedirectResponse(url=redirect_url, status_code=302)
    
    # Generate login form HTML
    oauth_login_button = ""
    if auth_config.method == AuthMethod.OAUTH and auth_config.oauth_provider:
        oauth_url = f"/auth/oauth/authorize"
        if next:
            oauth_url += f"?next={urllib.parse.quote(next)}"
        
        provider_name = auth_config.oauth_provider.value.title()
        oauth_login_button = f"""
        <div style="margin: 20px 0; text-align: center;">
            <a href="{oauth_url}" 
               style="display: inline-block; padding: 12px 24px; background: #4285f4; color: white; 
                      text-decoration: none; border-radius: 4px; font-weight: bold;">
                Login with {provider_name}
            </a>
        </div>
        """
    
    simple_login_form = ""
    if auth_config.method in [AuthMethod.SIMPLE, AuthMethod.TOKEN]:
        next_input = f'<input type="hidden" name="next" value="{next}">' if next else ""
        simple_login_form = f"""
        <form method="post" action="/auth/login" style="max-width: 300px; margin: 0 auto;">
            {next_input}
            <div style="margin-bottom: 15px;">
                <label for="username" style="display: block; margin-bottom: 5px;">Username:</label>
                <input type="text" id="username" name="username" required 
                       style="width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px;">
            </div>
            <div style="margin-bottom: 20px;">
                <label for="password" style="display: block; margin-bottom: 5px;">Password:</label>
                <input type="password" id="password" name="password" required 
                       style="width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px;">
            </div>
            <button type="submit" 
                    style="width: 100%; padding: 12px; background: #007bff; color: white; 
                           border: none; border-radius: 4px; font-weight: bold; cursor: pointer;">
                Login
            </button>
        </form>
        """
    
    html_content = f"""
    <!DOCTYPE html>
    <html>
    <head>
        <title>JuicyTrade - Login</title>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <style>
            body {{ font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }}
            .container {{ max-width: 400px; margin: 0 auto; background: white; 
                         padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }}
            h1 {{ text-align: center; color: #333; margin-bottom: 30px; }}
            .method-info {{ text-align: center; margin-bottom: 20px; color: #666; 
                           font-size: 14px; }}
        </style>
    </head>
    <body>
        <div class="container">
            <h1>JuicyTrade Login</h1>
            <div class="method-info">Authentication Method: {auth_config.method.value.title()}</div>
            {oauth_login_button}
            {simple_login_form}
        </div>
    </body>
    </html>
    """
    
    return HTMLResponse(content=html_content)


# OAuth endpoints
@auth_router.get("/oauth/authorize")
async def oauth_authorize(request: Request, next: Optional[str] = None):
    """Initiate OAuth authorization flow."""
    
    if auth_config.method != AuthMethod.OAUTH or not auth_config.oauth_provider:
        raise HTTPException(status_code=400, detail="OAuth not configured")
    
    oauth_config = auth_config.get_oauth_config()
    if not oauth_config:
        raise HTTPException(status_code=500, detail="OAuth configuration not found")
    
    # Generate state parameter for CSRF protection
    state = secrets.token_urlsafe(32)
    
    # Store state and redirect info
    oauth_states[state] = {
        "provider": auth_config.oauth_provider.value,
        "created_at": datetime.now(timezone.utc),
        "next": next
    }
    
    # Clean old states (older than 10 minutes)
    cutoff = datetime.now(timezone.utc) - timedelta(minutes=10)
    expired_states = [
        state_key for state_key, state_data in oauth_states.items()
        if state_data.get("created_at", datetime.min.replace(tzinfo=timezone.utc)) < cutoff
    ]
    for expired_state in expired_states:
        oauth_states.pop(expired_state, None)
    
    # Build authorization URL
    params = {
        "client_id": auth_config.oauth_client_id,
        "redirect_uri": auth_config.oauth_redirect_uri,
        "scope": " ".join(oauth_config["scopes"]),
        "state": state,
        "response_type": "code"
    }
    
    auth_url = oauth_config["authorization_url"] + "?" + urllib.parse.urlencode(params)
    
    return RedirectResponse(url=auth_url, status_code=302)


@auth_router.get("/oauth/callback")
async def oauth_callback(
    request: Request,
    response: Response,
    code: Optional[str] = None,
    state: Optional[str] = None,
    error: Optional[str] = None
):
    """Handle OAuth callback."""
    
    if error:
        logger.error(f"OAuth error: {error}")
        raise HTTPException(status_code=400, detail=f"OAuth error: {error}")
    
    if not code or not state:
        logger.error(f"Missing OAuth parameters")
        raise HTTPException(status_code=400, detail="Missing code or state parameter")
    
    # Validate state
    if state not in oauth_states:
        raise HTTPException(status_code=400, detail="Invalid state parameter")
    
    state_data = oauth_states.pop(state)
    provider = state_data["provider"]
    next_url = state_data.get("next", "/")
    
    # Get OAuth configuration
    oauth_config = auth_config.get_oauth_config()
    if not oauth_config:
        raise HTTPException(status_code=500, detail="OAuth configuration not found")
    
    try:
        # Exchange code for access token
        async with httpx.AsyncClient() as client:
            token_data = {
                "client_id": auth_config.oauth_client_id,
                "client_secret": auth_config.oauth_client_secret,
                "code": code,
                "grant_type": "authorization_code",
                "redirect_uri": auth_config.oauth_redirect_uri
            }
            
            token_response = await client.post(
                oauth_config["token_url"],
                data=token_data,
                headers={"Accept": "application/json"}
            )
            token_response.raise_for_status()
            
            tokens = token_response.json()
            access_token = tokens.get("access_token")
            
            if not access_token:
                raise HTTPException(status_code=500, detail="Failed to get access token")
            
            # Get user info
            user_response = await client.get(
                oauth_config["userinfo_url"],
                headers={"Authorization": f"Bearer {access_token}"}
            )
            user_response.raise_for_status()
            
            user_info = user_response.json()
            
            # Create user object
            user = User(
                username=user_info.get("login") or user_info.get("email") or str(user_info.get("id")),
                email=user_info.get("email"),
                full_name=user_info.get("name"),
                last_login=datetime.now(timezone.utc),
                metadata={
                    "oauth_provider": provider,
                    "oauth_user_id": str(user_info.get("id"))
                }
            )
            
            # Create session token
            session_token = create_access_token(user)
            expires_in = auth_config.jwt_expire_minutes * 60
            
            # Set session cookie with proper domain configuration
            cookie_kwargs = {
                "key": auth_config.session_cookie_name,
                "value": session_token,
                "max_age": expires_in,
                "httponly": False,  # Allow JavaScript access for frontend auth detection
                "secure": auth_config.secure_cookies,
                "samesite": "lax",  # Back to "lax" since "none" requires secure=true
                "path": "/"
            }
            
            # Set domain if configured (for production), otherwise no domain restriction
            if auth_config.cookie_domain:
                cookie_kwargs["domain"] = auth_config.cookie_domain
            # No else clause - if no domain is configured, don't set domain restriction
            
            response.set_cookie(**cookie_kwargs)
            
            # For development, also redirect with token in URL as fallback
            # Clean the next_url of any existing auth_token parameters
            # Handle case where next_url might be None
            if next_url and '?' in next_url:
                base_url, params = next_url.split('?', 1)
                # Remove any existing auth_token parameters
                param_pairs = [p for p in params.split('&') if not p.startswith('auth_token=')]
                if param_pairs:
                    clean_next_url = f"{base_url}?{'&'.join(param_pairs)}"
                else:
                    clean_next_url = base_url
            else:
                clean_next_url = next_url or "/"  # Default to "/" if next_url is None
            
            redirect_url = f"{clean_next_url}{'&' if '?' in clean_next_url else '?'}auth_token={session_token}"
            
            # Redirect to original destination
            return RedirectResponse(url=redirect_url, status_code=302)
    
    except httpx.HTTPError as e:
        logger.error(f"OAuth callback error: {e}")
        raise HTTPException(status_code=500, detail="OAuth authentication failed")
    except Exception as e:
        logger.error(f"OAuth callback error: {e}")
        raise HTTPException(status_code=500, detail="OAuth authentication failed")


@auth_router.get("/user", response_model=ApiResponse)
async def get_current_user_info(request: Request):
    """Get current user information."""
    user = get_current_user(request)
    
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Not authenticated"
        )
    
    return ApiResponse(
        success=True,
        data=user.model_dump(),
        message="User information retrieved"
    )
