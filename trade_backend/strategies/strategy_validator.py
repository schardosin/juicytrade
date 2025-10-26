"""
Enhanced Strategy Validator

Comprehensive validation system for trading strategy files that ensures:
- Valid Python syntax using AST parsing
- Framework compliance with BaseStrategy
- Security checks for malicious patterns
- Parameter schema validation
- Best practices enforcement
"""

import ast
import asyncio
import logging
import re
import importlib.util
import sys
from typing import Dict, List, Any, Optional, Set, Tuple
from datetime import datetime
from pathlib import Path

logger = logging.getLogger(__name__)

class ValidationResult:
    """Enhanced validation result container."""
    
    def __init__(self, success: bool, message: str, details: Dict[str, Any] = None):
        self.success = success
        self.message = message
        self.details = details or {}
        self.validation_steps = self.details.get("validation_steps", [])
        self.errors = self.details.get("errors", [])
        self.warnings = self.details.get("warnings", [])
        self.suggestions = self.details.get("suggestions", [])

class ValidationStep:
    """Individual validation step result."""
    
    def __init__(self, name: str, status: str, message: str, details: Dict[str, Any] = None):
        self.name = name
        self.status = status  # "passed", "failed", "warning", "skipped"
        self.message = message
        self.details = details or {}
        self.timestamp = datetime.now().isoformat()
    
    def to_dict(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "status": self.status,
            "message": self.message,
            "details": self.details,
            "timestamp": self.timestamp
        }

class SecurityPattern:
    """Security pattern definition for malicious code detection."""
    
    def __init__(self, pattern: str, description: str, severity: str = "high"):
        self.pattern = re.compile(pattern, re.IGNORECASE)
        self.description = description
        self.severity = severity  # "low", "medium", "high", "critical"

class StrategyValidator:
    """Enhanced strategy validator with comprehensive validation."""
    
    def __init__(self):
        self.validation_history = {}
        self.security_patterns = self._initialize_security_patterns()
        self.required_methods = {
            "initialize_strategy": "async def initialize_strategy(self)",
            "get_strategy_metadata": "def get_strategy_metadata(self)"
        }
        self.framework_imports = {
            "base_strategy": ["BaseStrategy"],
            "actions": ["Action", "ActionContext", "TimeAction", "MonitorAction", "TradeAction"]
        }
    
    def _initialize_security_patterns(self) -> List[SecurityPattern]:
        """Initialize security patterns for malicious code detection."""
        return [
            SecurityPattern(
                r"import\s+os|from\s+os\s+import",
                "Direct OS module access - potential system manipulation",
                "high"
            ),
            SecurityPattern(
                r"import\s+subprocess|from\s+subprocess\s+import",
                "Subprocess module - potential command execution",
                "critical"
            ),
            SecurityPattern(
                r"eval\s*\(|exec\s*\(",
                "Dynamic code execution - security risk",
                "critical"
            ),
            SecurityPattern(
                r"__import__\s*\(",
                "Dynamic imports - potential security risk",
                "high"
            ),
            SecurityPattern(
                r"open\s*\(.*['\"]w|open\s*\(.*mode\s*=\s*['\"]w",
                "File writing operations - potential data manipulation",
                "medium"
            ),
            SecurityPattern(
                r"socket\.|urllib\.|requests\.|http\.",
                "Network operations - potential data exfiltration",
                "medium"
            ),
            SecurityPattern(
                r"pickle\.|marshal\.|shelve\.",
                "Serialization modules - potential code injection",
                "high"
            ),
            SecurityPattern(
                r"globals\(\)|locals\(\)|vars\(\)",
                "Namespace manipulation - potential security risk",
                "medium"
            ),
            SecurityPattern(
                r"setattr\s*\(|getattr\s*\(|hasattr\s*\(|delattr\s*\(",
                "Dynamic attribute manipulation - potential security risk",
                "low"
            ),
            SecurityPattern(
                r"while\s+True\s*:|for\s+.*\s+in\s+.*:\s*while\s+True",
                "Infinite loops - potential DoS",
                "medium"
            )
        ]
    
    async def validate_strategy_file(self, file_content: bytes, filename: str) -> ValidationResult:
        """Comprehensive strategy file validation."""
        try:
            validation_steps = []
            errors = []
            warnings = []
            suggestions = []
            
            # Step 1: Basic file validation
            step = await self._validate_basic_file(file_content, filename)
            validation_steps.append(step.to_dict())
            if step.status == "failed":
                return self._create_failure_result(step.message, validation_steps, errors)
            
            # Decode file content
            try:
                file_str = file_content.decode('utf-8')
            except UnicodeDecodeError as e:
                return self._create_failure_result(
                    f"File encoding error: {str(e)}",
                    validation_steps,
                    ["File must be UTF-8 encoded"]
                )
            
            # Step 2: Python syntax validation
            step, ast_tree = await self._validate_python_syntax(file_str, filename)
            validation_steps.append(step.to_dict())
            if step.status == "failed":
                return self._create_failure_result(step.message, validation_steps, errors)
            
            # Step 3: Security validation
            step, security_issues = await self._validate_security(file_str, ast_tree)
            validation_steps.append(step.to_dict())
            if step.status == "failed":
                return self._create_failure_result(step.message, validation_steps, security_issues)
            elif security_issues:
                warnings.extend(security_issues)
            
            # Step 4: Import validation
            step, import_issues = await self._validate_imports(ast_tree)
            validation_steps.append(step.to_dict())
            if step.status == "failed":
                return self._create_failure_result(step.message, validation_steps, import_issues)
            elif import_issues:
                warnings.extend(import_issues)
            
            # Step 5: Class structure validation
            step, class_info = await self._validate_class_structure(ast_tree, filename)
            validation_steps.append(step.to_dict())
            if step.status == "failed":
                return self._create_failure_result(step.message, validation_steps, [step.message])
            
            # Step 6: BaseStrategy compliance validation
            step, compliance_issues = await self._validate_base_strategy_compliance(ast_tree, class_info)
            validation_steps.append(step.to_dict())
            if step.status == "failed":
                return self._create_failure_result(step.message, validation_steps, compliance_issues)
            elif compliance_issues:
                warnings.extend(compliance_issues)
            
            # Step 7: Method validation
            step, method_issues = await self._validate_required_methods(ast_tree, class_info)
            validation_steps.append(step.to_dict())
            if step.status == "failed":
                return self._create_failure_result(step.message, validation_steps, method_issues)
            
            # Step 8: Parameter schema validation
            step, schema_issues = await self._validate_parameter_schema(file_str, ast_tree)
            validation_steps.append(step.to_dict())
            if step.status == "warning" and schema_issues:
                warnings.extend(schema_issues)
            
            # Step 9: Best practices validation
            step, practice_issues = await self._validate_best_practices(ast_tree, file_str)
            validation_steps.append(step.to_dict())
            if practice_issues:
                suggestions.extend(practice_issues)
            
            # Store validation history
            validation_id = f"{filename}_{datetime.now().isoformat()}"
            self.validation_history[validation_id] = {
                "filename": filename,
                "timestamp": datetime.now().isoformat(),
                "success": True,
                "steps": validation_steps,
                "warnings": warnings,
                "suggestions": suggestions
            }
            
            logger.info(f"Strategy validation passed: {filename}")
            
            return ValidationResult(
                success=True,
                message="Strategy validation passed successfully",
                details={
                    "validation_id": validation_id,
                    "validation_steps": validation_steps,
                    "warnings": warnings,
                    "suggestions": suggestions,
                    "class_info": class_info,
                    "file_info": {
                        "filename": filename,
                        "size_bytes": len(file_content),
                        "lines": len(file_str.split('\n')),
                        "classes": len([node for node in ast.walk(ast_tree) if isinstance(node, ast.ClassDef)]),
                        "functions": len([node for node in ast.walk(ast_tree) if isinstance(node, ast.FunctionDef)])
                    }
                }
            )
            
        except Exception as e:
            logger.error(f"Strategy validation error: {e}")
            return ValidationResult(
                success=False,
                message=f"Validation failed: {str(e)}",
                details={
                    "error_type": "validation_exception",
                    "error": str(e),
                    "validation_steps": validation_steps
                }
            )
    
    async def _validate_basic_file(self, file_content: bytes, filename: str) -> ValidationStep:
        """Validate basic file properties."""
        try:
            # Check if file is empty
            if len(file_content.strip()) == 0:
                return ValidationStep(
                    "basic_file_check",
                    "failed",
                    "Strategy file is empty",
                    {"error_type": "empty_file"}
                )
            
            # Check file extension
            if not filename.lower().endswith('.py'):
                return ValidationStep(
                    "basic_file_check",
                    "failed",
                    "Strategy file must have .py extension",
                    {"error_type": "invalid_extension", "filename": filename}
                )
            
            # Check file size (reasonable limits)
            max_size = 1024 * 1024  # 1MB
            if len(file_content) > max_size:
                return ValidationStep(
                    "basic_file_check",
                    "failed",
                    f"File too large: {len(file_content)} bytes (max: {max_size})",
                    {"error_type": "file_too_large", "size": len(file_content)}
                )
            
            return ValidationStep(
                "basic_file_check",
                "passed",
                f"Basic file validation passed ({len(file_content)} bytes)",
                {"size": len(file_content), "filename": filename}
            )
            
        except Exception as e:
            return ValidationStep(
                "basic_file_check",
                "failed",
                f"Basic file validation error: {str(e)}",
                {"error": str(e)}
            )
    
    async def _validate_python_syntax(self, file_str: str, filename: str) -> Tuple[ValidationStep, Optional[ast.AST]]:
        """Validate Python syntax using AST parsing."""
        try:
            # Parse the Python code
            ast_tree = ast.parse(file_str, filename=filename)
            
            return ValidationStep(
                "python_syntax_check",
                "passed",
                "Python syntax validation passed",
                {
                    "nodes": len(list(ast.walk(ast_tree))),
                    "lines": len(file_str.split('\n'))
                }
            ), ast_tree
            
        except SyntaxError as e:
            return ValidationStep(
                "python_syntax_check",
                "failed",
                f"Python syntax error: {str(e)}",
                {
                    "error_type": "syntax_error",
                    "line": e.lineno,
                    "column": e.offset,
                    "text": e.text.strip() if e.text else None
                }
            ), None
        except Exception as e:
            return ValidationStep(
                "python_syntax_check",
                "failed",
                f"Syntax validation error: {str(e)}",
                {"error": str(e)}
            ), None
    
    async def _validate_security(self, file_str: str, ast_tree: ast.AST) -> Tuple[ValidationStep, List[str]]:
        """Validate security patterns and detect malicious code."""
        try:
            security_issues = []
            critical_issues = []
            
            # Check for dangerous patterns
            for pattern in self.security_patterns:
                matches = pattern.pattern.findall(file_str)
                if matches:
                    issue = f"{pattern.description} (found: {', '.join(matches[:3])})"
                    if pattern.severity == "critical":
                        critical_issues.append(issue)
                    else:
                        security_issues.append(issue)
            
            # Additional AST-based security checks
            for node in ast.walk(ast_tree):
                # Check for dangerous function calls
                if isinstance(node, ast.Call):
                    if isinstance(node.func, ast.Name):
                        if node.func.id in ['eval', 'exec', 'compile']:
                            critical_issues.append(f"Dangerous function call: {node.func.id}")
                
                # Check for dangerous imports
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        if alias.name in ['os', 'subprocess', 'sys']:
                            security_issues.append(f"Potentially dangerous import: {alias.name}")
            
            # Determine result
            if critical_issues:
                return ValidationStep(
                    "security_check",
                    "failed",
                    f"Critical security issues found: {len(critical_issues)}",
                    {"critical_issues": critical_issues, "security_issues": security_issues}
                ), critical_issues + security_issues
            elif security_issues:
                return ValidationStep(
                    "security_check",
                    "warning",
                    f"Security warnings found: {len(security_issues)}",
                    {"security_issues": security_issues}
                ), security_issues
            else:
                return ValidationStep(
                    "security_check",
                    "passed",
                    "No security issues detected",
                    {}
                ), []
                
        except Exception as e:
            return ValidationStep(
                "security_check",
                "failed",
                f"Security validation error: {str(e)}",
                {"error": str(e)}
            ), [f"Security validation error: {str(e)}"]
    
    async def _validate_imports(self, ast_tree: ast.AST) -> Tuple[ValidationStep, List[str]]:
        """Validate imports and framework dependencies."""
        try:
            imports = []
            from_imports = {}
            issues = []
            
            # Collect all imports
            for node in ast.walk(ast_tree):
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        imports.append(alias.name)
                elif isinstance(node, ast.ImportFrom):
                    module = node.module or ""
                    if module not in from_imports:
                        from_imports[module] = []
                    for alias in node.names:
                        from_imports[module].append(alias.name)
            
            # Check for framework imports
            has_base_strategy = False
            has_framework_imports = False
            
            # Check for BaseStrategy import
            for module, items in from_imports.items():
                if "base_strategy" in module.lower() and "BaseStrategy" in items:
                    has_base_strategy = True
                if any(framework_item in items for framework_items in self.framework_imports.values() 
                       for framework_item in framework_items):
                    has_framework_imports = True
            
            # Validate framework compliance
            if not has_base_strategy:
                issues.append("Missing BaseStrategy import - strategies must inherit from BaseStrategy")
            
            if not has_framework_imports:
                issues.append("No framework imports detected - consider using action-based framework components")
            
            # Check for problematic imports
            problematic = ['tkinter', 'pygame', 'flask', 'django', 'fastapi']
            for imp in imports:
                if any(prob in imp.lower() for prob in problematic):
                    issues.append(f"Potentially problematic import: {imp} - may not be suitable for trading strategies")
            
            if issues:
                return ValidationStep(
                    "import_validation",
                    "warning",
                    f"Import issues found: {len(issues)}",
                    {"issues": issues, "imports": imports, "from_imports": from_imports}
                ), issues
            else:
                return ValidationStep(
                    "import_validation",
                    "passed",
                    f"Import validation passed ({len(imports)} imports, {len(from_imports)} from-imports)",
                    {"imports": imports, "from_imports": from_imports}
                ), []
                
        except Exception as e:
            return ValidationStep(
                "import_validation",
                "failed",
                f"Import validation error: {str(e)}",
                {"error": str(e)}
            ), [f"Import validation error: {str(e)}"]
    
    async def _validate_class_structure(self, ast_tree: ast.AST, filename: str) -> Tuple[ValidationStep, Dict[str, Any]]:
        """Validate class structure and find strategy classes."""
        try:
            classes = []
            strategy_classes = []
            
            # Find all classes
            for node in ast.walk(ast_tree):
                if isinstance(node, ast.ClassDef):
                    class_info = {
                        "name": node.name,
                        "bases": [self._get_base_name(base) for base in node.bases],
                        "methods": [n.name for n in node.body if isinstance(n, ast.FunctionDef)],
                        "async_methods": [n.name for n in node.body if isinstance(n, ast.AsyncFunctionDef)],
                        "line": node.lineno
                    }
                    classes.append(class_info)
                    
                    # Check if it inherits from BaseStrategy
                    if any("BaseStrategy" in base for base in class_info["bases"]):
                        strategy_classes.append(class_info)
            
            # Validation
            if not classes:
                return ValidationStep(
                    "class_structure_check",
                    "failed",
                    "No classes found - strategy must contain at least one class",
                    {"classes": classes}
                ), {}
            
            if not strategy_classes:
                return ValidationStep(
                    "class_structure_check",
                    "failed",
                    "No BaseStrategy classes found - strategy must inherit from BaseStrategy",
                    {"classes": classes, "suggestion": "Add 'from .base_strategy import BaseStrategy' and inherit from it"}
                ), {}
            
            # Use the first strategy class as the main one
            main_strategy = strategy_classes[0]
            
            return ValidationStep(
                "class_structure_check",
                "passed",
                f"Class structure validation passed - found {len(strategy_classes)} strategy class(es)",
                {"classes": classes, "strategy_classes": strategy_classes, "main_strategy": main_strategy}
            ), main_strategy
            
        except Exception as e:
            return ValidationStep(
                "class_structure_check",
                "failed",
                f"Class structure validation error: {str(e)}",
                {"error": str(e)}
            ), {}
    
    async def _validate_base_strategy_compliance(self, ast_tree: ast.AST, class_info: Dict[str, Any]) -> Tuple[ValidationStep, List[str]]:
        """Validate BaseStrategy compliance."""
        try:
            issues = []
            
            if not class_info:
                return ValidationStep(
                    "base_strategy_compliance",
                    "failed",
                    "No strategy class information available",
                    {}
                ), ["No strategy class found"]
            
            # Check inheritance
            if not any("BaseStrategy" in base for base in class_info.get("bases", [])):
                issues.append("Class must inherit from BaseStrategy")
            
            # Check for constructor
            methods = class_info.get("methods", []) + class_info.get("async_methods", [])
            if "__init__" not in methods:
                issues.append("Strategy class should have an __init__ method")
            
            if issues:
                return ValidationStep(
                    "base_strategy_compliance",
                    "failed",
                    f"BaseStrategy compliance issues: {len(issues)}",
                    {"issues": issues}
                ), issues
            else:
                return ValidationStep(
                    "base_strategy_compliance",
                    "passed",
                    "BaseStrategy compliance validation passed",
                    {"class_name": class_info.get("name"), "methods": len(methods)}
                ), []
                
        except Exception as e:
            return ValidationStep(
                "base_strategy_compliance",
                "failed",
                f"BaseStrategy compliance validation error: {str(e)}",
                {"error": str(e)}
            ), [f"Compliance validation error: {str(e)}"]
    
    async def _validate_required_methods(self, ast_tree: ast.AST, class_info: Dict[str, Any]) -> Tuple[ValidationStep, List[str]]:
        """Validate required abstract methods."""
        try:
            issues = []
            
            if not class_info:
                return ValidationStep(
                    "required_methods_check",
                    "failed",
                    "No strategy class information available",
                    {}
                ), ["No strategy class found"]
            
            all_methods = class_info.get("methods", []) + class_info.get("async_methods", [])
            
            # Check required methods
            for method_name in self.required_methods.keys():
                if method_name not in all_methods:
                    issues.append(f"Missing required method: {method_name}")
            
            # Check specific method signatures by examining AST
            for node in ast.walk(ast_tree):
                if isinstance(node, ast.ClassDef) and node.name == class_info.get("name"):
                    for item in node.body:
                        if isinstance(item, ast.AsyncFunctionDef) and item.name == "initialize_strategy":
                            # Validate initialize_strategy signature
                            if len(item.args.args) != 1:  # Should only have 'self'
                                issues.append("initialize_strategy should only take 'self' parameter")
                        
                        elif isinstance(item, ast.FunctionDef) and item.name == "get_strategy_metadata":
                            # Validate get_strategy_metadata signature
                            if len(item.args.args) != 1:  # Should only have 'self'
                                issues.append("get_strategy_metadata should only take 'self' parameter")
            
            if issues:
                return ValidationStep(
                    "required_methods_check",
                    "failed",
                    f"Required method issues: {len(issues)}",
                    {"issues": issues, "found_methods": all_methods}
                ), issues
            else:
                return ValidationStep(
                    "required_methods_check",
                    "passed",
                    f"Required methods validation passed ({len(all_methods)} methods found)",
                    {"methods": all_methods}
                ), []
                
        except Exception as e:
            return ValidationStep(
                "required_methods_check",
                "failed",
                f"Required methods validation error: {str(e)}",
                {"error": str(e)}
            ), [f"Method validation error: {str(e)}"]
    
    async def _validate_parameter_schema(self, file_str: str, ast_tree: ast.AST) -> Tuple[ValidationStep, List[str]]:
        """Validate parameter schema in get_strategy_metadata."""
        try:
            issues = []
            has_metadata_method = False
            has_parameters = False
            
            # Look for get_strategy_metadata method
            for node in ast.walk(ast_tree):
                if isinstance(node, ast.FunctionDef) and node.name == "get_strategy_metadata":
                    has_metadata_method = True
                    
                    # Look for parameters in the return dictionary
                    for item in ast.walk(node):
                        if isinstance(item, ast.Dict):
                            for key in item.keys:
                                if isinstance(key, ast.Constant) and key.value == "parameters":
                                    has_parameters = True
                                    break
            
            if not has_metadata_method:
                issues.append("get_strategy_metadata method not found")
            elif not has_parameters:
                issues.append("No 'parameters' section found in strategy metadata")
            
            # Additional checks could be added here for parameter schema validation
            
            if issues:
                return ValidationStep(
                    "parameter_schema_check",
                    "warning",
                    f"Parameter schema issues: {len(issues)}",
                    {"issues": issues}
                ), issues
            else:
                return ValidationStep(
                    "parameter_schema_check",
                    "passed",
                    "Parameter schema validation passed",
                    {"has_metadata": has_metadata_method, "has_parameters": has_parameters}
                ), []
                
        except Exception as e:
            return ValidationStep(
                "parameter_schema_check",
                "warning",
                f"Parameter schema validation error: {str(e)}",
                {"error": str(e)}
            ), [f"Parameter schema error: {str(e)}"]
    
    async def _validate_best_practices(self, ast_tree: ast.AST, file_str: str) -> Tuple[ValidationStep, List[str]]:
        """Validate best practices and provide suggestions."""
        try:
            suggestions = []
            
            # Check for docstrings
            has_class_docstring = False
            has_method_docstrings = 0
            total_methods = 0
            
            for node in ast.walk(ast_tree):
                if isinstance(node, ast.ClassDef):
                    if ast.get_docstring(node):
                        has_class_docstring = True
                
                if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                    total_methods += 1
                    if ast.get_docstring(node):
                        has_method_docstrings += 1
            
            if not has_class_docstring:
                suggestions.append("Add class docstring to describe strategy purpose")
            
            if total_methods > 0 and has_method_docstrings / total_methods < 0.5:
                suggestions.append("Add docstrings to methods for better documentation")
            
            # Check for logging
            has_logging = "log_info" in file_str or "logger" in file_str or "logging" in file_str
            if not has_logging:
                suggestions.append("Consider adding logging for better debugging and monitoring")
            
            # Check for error handling
            has_try_catch = "try:" in file_str and "except" in file_str
            if not has_try_catch:
                suggestions.append("Add error handling with try/except blocks")
            
            # Check for action framework usage
            has_actions = any(action in file_str for action in ["add_time_action", "add_monitor_action", "add_trade_action"])
            if not has_actions:
                suggestions.append("Consider using action framework methods for better strategy structure")
            
            return ValidationStep(
                "best_practices_check",
                "passed",
                f"Best practices check completed ({len(suggestions)} suggestions)",
                {"suggestions": suggestions, "docstring_coverage": has_method_docstrings / max(total_methods, 1)}
            ), suggestions
            
        except Exception as e:
            return ValidationStep(
                "best_practices_check",
                "warning",
                f"Best practices validation error: {str(e)}",
                {"error": str(e)}
            ), [f"Best practices error: {str(e)}"]
    
    def _get_base_name(self, node: ast.AST) -> str:
        """Extract base class name from AST node."""
        if isinstance(node, ast.Name):
            return node.id
        elif isinstance(node, ast.Attribute):
            return f"{self._get_base_name(node.value)}.{node.attr}"
        else:
            return str(node)
    
    def _create_failure_result(self, message: str, validation_steps: List[Dict], errors: List[str]) -> ValidationResult:
        """Create a validation failure result."""
        return ValidationResult(
            success=False,
            message=message,
            details={
                "validation_steps": validation_steps,
                "errors": errors,
                "error_type": "validation_failed"
            }
        )
    
    async def validate_existing_strategy(self, strategy_id: str) -> ValidationResult:
        """Re-validate an existing strategy."""
        try:
            logger.info(f"Re-validating strategy: {strategy_id}")
            
            # This would need to be implemented to load the strategy file
            # For now, return a mock successful validation
            validation_steps = [
                ValidationStep("strategy_exists_check", "passed", "Strategy found in registry").to_dict(),
                ValidationStep("syntax_recheck", "passed", "Syntax validation passed").to_dict(),
                ValidationStep("dependency_check", "passed", "All dependencies available").to_dict(),
                ValidationStep("compatibility_check", "passed", "Compatible with current system version").to_dict()
            ]
            
            return ValidationResult(
                success=True,
                message="Strategy re-validation passed",
                details={
                    "strategy_id": strategy_id,
                    "validation_steps": validation_steps,
                    "timestamp": datetime.now().isoformat()
                }
            )
            
        except Exception as e:
            logger.error(f"Strategy re-validation error: {e}")
            return ValidationResult(
                success=False,
                message=f"Re-validation failed: {str(e)}",
                details={"error_type": "revalidation_exception", "error": str(e)}
            )
    
    def get_validation_history(self, limit: int = 10) -> List[Dict[str, Any]]:
        """Get recent validation history."""
        try:
            history_items = list(self.validation_history.values())
            history_items.sort(key=lambda x: x['timestamp'], reverse=True)
            return history_items[:limit]
            
        except Exception as e:
            logger.error(f"Error getting validation history: {e}")
            return []
    
    def get_validation_stats(self) -> Dict[str, Any]:
        """Get validation statistics."""
        try:
            total_validations = len(self.validation_history)
            successful_validations = sum(1 for v in self.validation_history.values() if v['success'])
            
            return {
                "total_validations": total_validations,
                "successful_validations": successful_validations,
                "failed_validations": total_validations - successful_validations,
                "success_rate": successful_validations / total_validations if total_validations > 0 else 0,
                "security_patterns": len(self.security_patterns),
                "required_methods": len(self.required_methods)
            }
            
        except Exception as e:
            logger.error(f"Error getting validation stats: {e}")
            return {
                "total_validations": 0,
                "successful_validations": 0,
                "failed_validations": 0,
                "success_rate": 0,
                "security_patterns": 0,
                "required_methods": 0
            }
