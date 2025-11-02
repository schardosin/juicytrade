"""
Authentication configuration for juicytrade.
Supports multiple authentication methods: simple, oauth, token, or disabled.
"""
import os
from enum import Enum
from typing import Optional, Dict, Any
from pydantic import BaseModel, Field


class AuthMethod(str, Enum):
    """Supported authentication methods."""
    DISABLED = "disabled"
    SIMPLE = "simple"
    OAUTH = "oauth"
    TOKEN = "token"
    HEADER = "header"


class OAuthProvider(str, Enum):
    """Supported OAuth providers."""
    GOOGLE = "google"
    GITHUB = "github"
    MICROSOFT = "microsoft"


class AuthConfig(BaseModel):
    """Authentication configuration settings."""
    
    # Main authentication method
    method: AuthMethod = Field(
        default=AuthMethod.DISABLED,
        description="Authentication method to use"
    )
    
    # Simple authentication settings
    simple_username: Optional[str] = Field(
        default=None,
        description="Username for simple authentication"
    )
    simple_password: Optional[str] = Field(
        default=None,
        description="Password for simple authentication"
    )
    
    # OAuth settings
    oauth_provider: Optional[OAuthProvider] = Field(
        default=None,
        description="OAuth provider to use"
    )
    oauth_client_id: Optional[str] = Field(
        default=None,
        description="OAuth client ID"
    )
    oauth_client_secret: Optional[str] = Field(
        default=None,
        description="OAuth client secret"
    )
    oauth_redirect_uri: Optional[str] = Field(
        default=None,
        description="OAuth redirect URI"
    )
    
    # JWT settings
    jwt_secret_key: str = Field(
        default="juicytrade-default-secret-key-change-in-production",
        description="JWT secret key for token signing"
    )
    jwt_expire_minutes: int = Field(
        default=1440,  # 24 hours
        description="JWT token expiration time in minutes"
    )
    jwt_algorithm: str = Field(
        default="HS256",
        description="JWT signing algorithm"
    )
    
    # Header-based authentication
    header_name: str = Field(
        default="X-Remote-User",
        description="Header name for header-based authentication"
    )
    
    # Session settings
    session_cookie_name: str = Field(
        default="juicytrade_session",
        description="Session cookie name"
    )
    session_max_age: int = Field(
        default=86400,  # 24 hours
        description="Session max age in seconds"
    )
    
    # Security settings
    enable_csrf_protection: bool = Field(
        default=True,
        description="Enable CSRF protection"
    )
    secure_cookies: bool = Field(
        default=False,
        description="Use secure cookies (HTTPS only)"
    )
    cookie_domain: Optional[str] = Field(
        default=None,
        description="Cookie domain for session cookies"
    )
    
    # User authorization settings
    oauth_allowed_emails: Optional[str] = Field(
        default=None,
        description="Comma-separated list of allowed email addresses for OAuth"
    )
    oauth_allowed_domains: Optional[str] = Field(
        default=None,
        description="Comma-separated list of allowed email domains for OAuth"
    )
    
    @classmethod
    def from_env(cls) -> "AuthConfig":
        """Create AuthConfig from environment variables."""
        return cls(
            method=AuthMethod(os.getenv("AUTH_METHOD", "disabled").lower()),
            simple_username=os.getenv("AUTH_SIMPLE_USERNAME"),
            simple_password=os.getenv("AUTH_SIMPLE_PASSWORD"),
            oauth_provider=OAuthProvider(os.getenv("AUTH_OAUTH_PROVIDER", "google").lower()) if os.getenv("AUTH_OAUTH_PROVIDER") else None,
            oauth_client_id=os.getenv("AUTH_OAUTH_CLIENT_ID"),
            oauth_client_secret=os.getenv("AUTH_OAUTH_CLIENT_SECRET"),
            oauth_redirect_uri=os.getenv("AUTH_OAUTH_REDIRECT_URI"),
            jwt_secret_key=os.getenv("AUTH_JWT_SECRET_KEY", "juicytrade-default-secret-key-change-in-production"),
            jwt_expire_minutes=int(os.getenv("AUTH_JWT_EXPIRE_MINUTES", "1440")),
            jwt_algorithm=os.getenv("AUTH_JWT_ALGORITHM", "HS256"),
            header_name=os.getenv("AUTH_HEADER_NAME", "X-Remote-User"),
            session_cookie_name=os.getenv("AUTH_SESSION_COOKIE_NAME", "juicytrade_session"),
            session_max_age=int(os.getenv("AUTH_SESSION_MAX_AGE", "86400")),
            enable_csrf_protection=os.getenv("AUTH_ENABLE_CSRF", "true").lower() == "true",
            secure_cookies=os.getenv("AUTH_SECURE_COOKIES", "false").lower() == "true",
            cookie_domain=os.getenv("AUTH_COOKIE_DOMAIN"),
            oauth_allowed_emails=os.getenv("AUTH_OAUTH_ALLOWED_EMAILS"),
            oauth_allowed_domains=os.getenv("AUTH_OAUTH_ALLOWED_DOMAINS")
        )
    
    def is_enabled(self) -> bool:
        """Check if authentication is enabled."""
        return self.method != AuthMethod.DISABLED
    
    def validate_config(self) -> tuple[bool, list[str]]:
        """Validate the authentication configuration."""
        errors = []
        
        if not self.is_enabled():
            return True, []
        
        if self.method == AuthMethod.SIMPLE:
            if not self.simple_username:
                errors.append("AUTH_SIMPLE_USERNAME is required for simple authentication")
            if not self.simple_password:
                errors.append("AUTH_SIMPLE_PASSWORD is required for simple authentication")
        
        elif self.method == AuthMethod.OAUTH:
            if not self.oauth_provider:
                errors.append("AUTH_OAUTH_PROVIDER is required for OAuth authentication")
            if not self.oauth_client_id:
                errors.append("AUTH_OAUTH_CLIENT_ID is required for OAuth authentication")
            if not self.oauth_client_secret:
                errors.append("AUTH_OAUTH_CLIENT_SECRET is required for OAuth authentication")
            if not self.oauth_redirect_uri:
                errors.append("AUTH_OAUTH_REDIRECT_URI is required for OAuth authentication")
        
        elif self.method == AuthMethod.TOKEN:
            if self.jwt_secret_key == "juicytrade-default-secret-key-change-in-production":
                errors.append("AUTH_JWT_SECRET_KEY should be changed from default value in production")
        
        return len(errors) == 0, errors
    
    def get_oauth_config(self) -> Optional[Dict[str, Any]]:
        """Get OAuth provider configuration."""
        if self.method != AuthMethod.OAUTH or not self.oauth_provider:
            return None
        
        oauth_configs = {
            OAuthProvider.GOOGLE: {
                "authorization_url": "https://accounts.google.com/o/oauth2/auth",
                "token_url": "https://oauth2.googleapis.com/token",
                "userinfo_url": "https://www.googleapis.com/oauth2/v2/userinfo",
                "scopes": ["openid", "email", "profile"]
            },
            OAuthProvider.GITHUB: {
                "authorization_url": "https://github.com/login/oauth/authorize",
                "token_url": "https://github.com/login/oauth/access_token",
                "userinfo_url": "https://api.github.com/user",
                "scopes": ["user:email"]
            },
            OAuthProvider.MICROSOFT: {
                "authorization_url": "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
                "token_url": "https://login.microsoftonline.com/common/oauth2/v2.0/token",
                "userinfo_url": "https://graph.microsoft.com/v1.0/me",
                "scopes": ["openid", "profile", "email"]
            }
        }
        
        return oauth_configs.get(self.oauth_provider)
    
    def is_user_authorized(self, email: str) -> bool:
        """Check if a user email is authorized for OAuth access."""
        if not email:
            return False
        
        email = email.lower().strip()
        
        # Check allowed emails list
        if self.oauth_allowed_emails:
            allowed_emails = [e.lower().strip() for e in self.oauth_allowed_emails.split(',') if e.strip()]
            if email in allowed_emails:
                return True
        
        # Check allowed domains list
        if self.oauth_allowed_domains:
            email_domain = email.split('@')[-1] if '@' in email else ''
            allowed_domains = [d.lower().strip() for d in self.oauth_allowed_domains.split(',') if d.strip()]
            if email_domain in allowed_domains:
                return True
        
        # If no restrictions are configured, allow all users (backward compatibility)
        if not self.oauth_allowed_emails and not self.oauth_allowed_domains:
            return True
        
        # User is not authorized
        return False


# Global auth config instance
auth_config = AuthConfig.from_env()
