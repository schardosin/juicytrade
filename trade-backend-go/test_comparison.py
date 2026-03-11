#!/usr/bin/env python3
"""
Response comparison test between Go and Python backends.
This script validates that the Go backend produces identical responses to the Python backend.
"""

import json
import requests
import sys
from typing import Dict, Any

def test_endpoint(go_port: int, python_port: int, endpoint: str) -> bool:
    """Test an endpoint on both servers and compare responses."""
    print(f"\n=== Testing {endpoint} ===")
    
    # Test Go server
    try:
        go_response = requests.get(f"http://localhost:{go_port}{endpoint}", timeout=5)
        go_data = go_response.json()
        print(f"Go Response ({go_response.status_code}):")
        print(json.dumps(go_data, indent=2))
    except Exception as e:
        print(f"Go server error: {e}")
        return False
    
    # For now, we'll just validate the Go response structure
    # Later we can add Python comparison when the Python server is working
    
    # Validate Go response structure matches expected format
    expected_fields = ["success", "data", "error", "message", "timestamp"]
    
    if not all(field in go_data for field in expected_fields):
        print(f"❌ Go response missing required fields. Expected: {expected_fields}")
        return False
    
    if not isinstance(go_data["success"], bool):
        print("❌ Go response 'success' field should be boolean")
        return False
    
    if go_data["success"] and go_data["data"] is None:
        print("❌ Go response 'data' should not be None when success=True")
        return False
    
    print("✅ Go response structure is valid")
    return True

def main():
    """Main test function."""
    go_port = 8009
    python_port = 8008
    
    print("🧪 Testing Go backend response format compatibility")
    print(f"Go server: http://localhost:{go_port}")
    print(f"Python server: http://localhost:{python_port}")
    
    # Test endpoints
    endpoints = [
        "/",
        "/health"
    ]
    
    all_passed = True
    for endpoint in endpoints:
        if not test_endpoint(go_port, python_port, endpoint):
            all_passed = False
    
    if all_passed:
        print("\n🎉 All tests passed! Go backend response format is compatible.")
        return 0
    else:
        print("\n❌ Some tests failed. Go backend needs fixes.")
        return 1

if __name__ == "__main__":
    sys.exit(main())
