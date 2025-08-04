import json
import os
import logging
import time
from typing import Dict, List, Optional, Any

logger = logging.getLogger(__name__)

class ProviderCredentialStore:
    def __init__(self, credentials_file: str = "provider_credentials.json"):
        self.credentials_file = credentials_file
        self._data = self._load_credentials()
    
    def _load_credentials(self) -> Dict[str, Any]:
        """Load credentials from JSON file"""
        if os.path.exists(self.credentials_file):
            try:
                with open(self.credentials_file, 'r') as f:
                    data = json.load(f)
                    logger.info(f"✅ Loaded provider credentials from {self.credentials_file}")
                    return data
            except (json.JSONDecodeError, IOError) as e:
                logger.error(f"❌ Error loading {self.credentials_file}: {e}")
                return {}
        else:
            logger.info(f"📝 Creating new credentials file: {self.credentials_file}")
            return {}
    
    def _save_credentials(self):
        """Save credentials to JSON file"""
        try:
            with open(self.credentials_file, 'w') as f:
                json.dump(self._data, f, indent=2)
            logger.info(f"💾 Saved provider credentials to {self.credentials_file}")
        except IOError as e:
            logger.error(f"❌ Error saving {self.credentials_file}: {e}")
            raise
    
    def get_all_instances(self) -> Dict[str, Any]:
        """Get all provider instances"""
        return self._data.copy()
    
    def get_active_instances(self) -> Dict[str, Any]:
        """Get only active provider instances"""
        return {
            instance_id: instance_data 
            for instance_id, instance_data in self._data.items() 
            if instance_data.get('active', False)
        }
    
    def get_instance(self, instance_id: str) -> Optional[Dict[str, Any]]:
        """Get a specific provider instance"""
        return self._data.get(instance_id)
    
    def add_instance(self, instance_id: str, provider_type: str, account_type: str, 
                    display_name: str, credentials: Dict[str, str]) -> bool:
        """Add a new provider instance"""
        try:
            self._data[instance_id] = {
                'active': True,
                'provider_type': provider_type,
                'account_type': account_type,
                'display_name': display_name,
                'credentials': credentials,
                'created_at': int(time.time()),
                'updated_at': int(time.time())
            }
            self._save_credentials()
            logger.info(f"➕ Added provider instance: {instance_id}")
            return True
        except Exception as e:
            logger.error(f"❌ Error adding provider instance {instance_id}: {e}")
            return False
    
    def update_instance(self, instance_id: str, **updates) -> bool:
        """Update an existing provider instance"""
        try:
            if instance_id not in self._data:
                logger.warning(f"⚠️ Provider instance not found: {instance_id}")
                return False
            
            updates['updated_at'] = int(time.time())
            self._data[instance_id].update(updates)
            self._save_credentials()
            logger.info(f"✏️ Updated provider instance: {instance_id}")
            return True
        except Exception as e:
            logger.error(f"❌ Error updating provider instance {instance_id}: {e}")
            return False
    
    def delete_instance(self, instance_id: str) -> bool:
        """Delete a provider instance"""
        try:
            if instance_id in self._data:
                del self._data[instance_id]
                self._save_credentials()
                logger.info(f"🗑️ Deleted provider instance: {instance_id}")
                return True
            else:
                logger.warning(f"⚠️ Provider instance not found for deletion: {instance_id}")
                return False
        except Exception as e:
            logger.error(f"❌ Error deleting provider instance {instance_id}: {e}")
            return False
    
    def toggle_instance(self, instance_id: str) -> Optional[bool]:
        """Toggle active status of a provider instance"""
        try:
            if instance_id not in self._data:
                logger.warning(f"⚠️ Provider instance not found: {instance_id}")
                return None
            
            current_active = self._data[instance_id].get('active', False)
            new_active = not current_active
            
            self._data[instance_id]['active'] = new_active
            self._data[instance_id]['updated_at'] = int(time.time())
            self._save_credentials()
            
            logger.info(f"🔄 Toggled provider instance {instance_id}: {current_active} → {new_active}")
            return new_active
        except Exception as e:
            logger.error(f"❌ Error toggling provider instance {instance_id}: {e}")
            return None
    
    def get_instances_by_type(self, provider_type: str) -> Dict[str, Any]:
        """Get all instances of a specific provider type"""
        return {
            instance_id: instance_data
            for instance_id, instance_data in self._data.items()
            if instance_data.get('provider_type') == provider_type
        }
    
    def validate_instance_id(self, instance_id: str) -> bool:
        """Check if instance ID is unique"""
        return instance_id not in self._data
    
    def generate_instance_id(self, provider_type: str, account_type: str, display_name: str = None) -> str:
        """Generate a unique instance ID"""
        base_name = display_name.lower().replace(' ', '_').replace('(', '').replace(')', '') if display_name else f"{provider_type}_{account_type}"
        base_id = f"{provider_type}_{account_type}_{base_name}"
        
        # Ensure uniqueness
        counter = 1
        instance_id = base_id
        while not self.validate_instance_id(instance_id):
            instance_id = f"{base_id}_{counter}"
            counter += 1
        
        return instance_id
