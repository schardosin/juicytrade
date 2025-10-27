"""
Authentication models for juicytrade.
"""
from datetime import datetime
from typing import Optional, Dict, Any
from pydantic import BaseModel, Field


class User(BaseModel):
    """User model for authentication."""
    username: str = Field(..., description="Username")
    email: Optional[str] = Field(None, description="User email")
    full_name: Optional[str] = Field(None, description="Full name")
    is_active: bool = Field(True, description="Is user active")
    created_at: Optional[datetime] = Field(None, description="User creation timestamp")
    last_login: Optional[datetime] = Field(None, description="Last login timestamp")
    metadata: Optional[Dict[str, Any]] = Field(None, description="Additional user metadata")


class LoginRequest(BaseModel):
    """Login request model."""
    username: str = Field(..., description="Username")
    password: str = Field(..., description="Password")


class LoginResponse(BaseModel):
    """Login response model."""
    success: bool = Field(..., description="Login success status")
    message: str = Field(..., description="Login message")
    access_token: Optional[str] = Field(None, description="JWT access token")
    token_type: str = Field("bearer", description="Token type")
    expires_in: Optional[int] = Field(None, description="Token expiration in seconds")
    user: Optional[User] = Field(None, description="User information")


class LogoutResponse(BaseModel):
    """Logout response model."""
    success: bool = Field(..., description="Logout success status")
    message: str = Field(..., description="Logout message")


class AuthStatus(BaseModel):
    """Authentication status model."""
    authenticated: bool = Field(..., description="Is user authenticated")
    method: str = Field(..., description="Authentication method used")
    user: Optional[User] = Field(None, description="Current user")
    expires_at: Optional[datetime] = Field(None, description="Token expiration time")


class OAuthState(BaseModel):
    """OAuth state model for tracking OAuth flows."""
    state: str = Field(..., description="OAuth state parameter")
    provider: str = Field(..., description="OAuth provider")
    redirect_uri: str = Field(..., description="OAuth redirect URI")
    created_at: datetime = Field(default_factory=datetime.utcnow, description="State creation time")


class TokenData(BaseModel):
    """JWT token data model."""
    username: str = Field(..., description="Username")
    expires_at: datetime = Field(..., description="Token expiration time")
    issued_at: datetime = Field(default_factory=datetime.utcnow, description="Token issued time")
    metadata: Optional[Dict[str, Any]] = Field(None, description="Additional token metadata")
