"""
Time Management System for Strategy Framework

This module provides comprehensive time management for trading strategies:
- Market hours awareness (NYSE, NASDAQ, options markets)
- Time-based scheduling and triggers
- Trading session management
- Holiday calendar integration
- Time zone handling for global markets
"""

import logging
from datetime import datetime, time, date, timedelta
from typing import Dict, List, Optional, Set, Tuple, Union
from enum import Enum
from dataclasses import dataclass
import pytz
from zoneinfo import ZoneInfo

logger = logging.getLogger(__name__)

# ============================================================================
# Market and Time Zone Definitions
# ============================================================================

class MarketType(Enum):
    """Types of financial markets"""
    STOCK = "stock"
    OPTIONS = "options"
    FUTURES = "futures"
    FOREX = "forex"
    CRYPTO = "crypto"

class TradingSession(Enum):
    """Trading session types"""
    PRE_MARKET = "pre_market"
    REGULAR = "regular"
    AFTER_HOURS = "after_hours"
    CLOSED = "closed"

@dataclass
class MarketHours:
    """Market hours definition"""
    market_type: MarketType
    timezone: str
    pre_market_open: Optional[time] = None
    regular_open: time = time(9, 30)
    regular_close: time = time(16, 0)
    after_hours_close: Optional[time] = None
    
    def __post_init__(self):
        """Validate market hours"""
        if self.regular_open >= self.regular_close:
            raise ValueError("Regular market open must be before close")

@dataclass
class TradingDay:
    """Represents a trading day with sessions"""
    date: date
    market_type: MarketType
    is_trading_day: bool
    sessions: Dict[TradingSession, Tuple[datetime, datetime]]
    early_close: bool = False
    late_open: bool = False
    notes: str = ""

# ============================================================================
# Market Hours Configuration
# ============================================================================

# US Market Hours (Eastern Time)
US_STOCK_HOURS = MarketHours(
    market_type=MarketType.STOCK,
    timezone="America/New_York",
    pre_market_open=time(4, 0),
    regular_open=time(9, 30),
    regular_close=time(16, 0),
    after_hours_close=time(20, 0)
)

US_OPTIONS_HOURS = MarketHours(
    market_type=MarketType.OPTIONS,
    timezone="America/New_York",
    pre_market_open=time(4, 0),
    regular_open=time(9, 30),
    regular_close=time(16, 0),
    after_hours_close=time(17, 15)  # Options after-hours shorter
)

# Market configurations
MARKET_CONFIGS = {
    MarketType.STOCK: US_STOCK_HOURS,
    MarketType.OPTIONS: US_OPTIONS_HOURS,
}

# US Market Holidays (2024-2025)
US_MARKET_HOLIDAYS = {
    # 2024
    date(2024, 1, 1),   # New Year's Day
    date(2024, 1, 15),  # Martin Luther King Jr. Day
    date(2024, 2, 19),  # Presidents' Day
    date(2024, 3, 29),  # Good Friday
    date(2024, 5, 27),  # Memorial Day
    date(2024, 6, 19),  # Juneteenth
    date(2024, 7, 4),   # Independence Day
    date(2024, 9, 2),   # Labor Day
    date(2024, 11, 28), # Thanksgiving
    date(2024, 12, 25), # Christmas
    
    # 2025
    date(2025, 1, 1),   # New Year's Day
    date(2025, 1, 20),  # Martin Luther King Jr. Day
    date(2025, 2, 17),  # Presidents' Day
    date(2025, 4, 18),  # Good Friday
    date(2025, 5, 26),  # Memorial Day
    date(2025, 6, 19),  # Juneteenth
    date(2025, 7, 4),   # Independence Day
    date(2025, 9, 1),   # Labor Day
    date(2025, 11, 27), # Thanksgiving
    date(2025, 12, 25), # Christmas
}

# Early close days (1:00 PM ET)
US_EARLY_CLOSE_DAYS = {
    date(2024, 7, 3),   # Day before Independence Day
    date(2024, 11, 29), # Day after Thanksgiving
    date(2024, 12, 24), # Christmas Eve
    
    date(2025, 7, 3),   # Day before Independence Day
    date(2025, 11, 28), # Day after Thanksgiving
    date(2025, 12, 24), # Christmas Eve
}

# ============================================================================
# Time Scheduler
# ============================================================================

class TimeScheduler:
    """
    Advanced time scheduler for strategy actions with market awareness
    """
    
    def __init__(self, market_type: MarketType = MarketType.STOCK):
        self.market_type = market_type
        self.market_hours = MARKET_CONFIGS.get(market_type, US_STOCK_HOURS)
        self.timezone = ZoneInfo(self.market_hours.timezone)
        
        # Scheduled events
        self.scheduled_events: List[Dict] = []
        self.recurring_events: List[Dict] = []

        # ARCHITECTURAL FIX: Override time for backtesting/testing
        self._override_time: Optional[datetime] = None
        
        logger.info(f"TimeScheduler initialized for {market_type.value} market")
    
    def get_current_time(self) -> datetime:
        """Get current time in market timezone (respects override for backtesting)"""
        if self._override_time is not None:
            # Return override time with proper timezone
            if self._override_time.tzinfo is None:
                # Assume override time is in market timezone if no timezone specified
                return self._override_time.replace(tzinfo=self.timezone)
            elif self._override_time.tzinfo != self.timezone:
                # Convert to market timezone
                return self._override_time.astimezone(self.timezone)
            else:
                return self._override_time

        # Default: return actual current time
        return datetime.now(self.timezone)

    def set_current_time(self, override_time: datetime) -> None:
        """
        Set override time for backtesting/testing scenarios.

        Args:
            override_time: The time to use instead of actual current time
        """
        self._override_time = override_time
        logger.debug(f"TimeScheduler override time set to: {override_time}")

    def clear_current_time(self) -> None:
        """Clear time override and return to using actual current time"""
        self._override_time = None
        logger.debug("TimeScheduler override time cleared")

    def is_time_overridden(self) -> bool:
        """Check if time is currently overridden for testing/backtesting"""
        return self._override_time is not None
    
    def get_current_session(self, dt: Optional[datetime] = None) -> TradingSession:
        """Get current trading session"""
        if dt is None:
            dt = self.get_current_time()
        
        # Convert to market timezone if needed
        if dt.tzinfo != self.timezone:
            dt = dt.astimezone(self.timezone)
        
        trading_day = self.get_trading_day(dt.date())
        
        if not trading_day.is_trading_day:
            return TradingSession.CLOSED
        
        current_time = dt.time()
        
        # Check each session
        for session, (start_dt, end_dt) in trading_day.sessions.items():
            if start_dt.time() <= current_time <= end_dt.time():
                return session
        
        return TradingSession.CLOSED
    
    def get_trading_day(self, target_date: date) -> TradingDay:
        """Get trading day information for a specific date"""
        # Check if it's a weekend
        is_weekend = target_date.weekday() >= 5  # Saturday = 5, Sunday = 6
        
        # Check if it's a holiday
        is_holiday = target_date in US_MARKET_HOLIDAYS
        
        # Check if it's an early close day
        is_early_close = target_date in US_EARLY_CLOSE_DAYS
        
        is_trading_day = not (is_weekend or is_holiday)
        
        sessions = {}
        
        if is_trading_day:
            # Create datetime objects for the trading day
            base_dt = datetime.combine(target_date, time(0, 0), tzinfo=self.timezone)
            
            # Pre-market session
            if self.market_hours.pre_market_open:
                pre_start = base_dt.replace(
                    hour=self.market_hours.pre_market_open.hour,
                    minute=self.market_hours.pre_market_open.minute
                )
                pre_end = base_dt.replace(
                    hour=self.market_hours.regular_open.hour,
                    minute=self.market_hours.regular_open.minute
                )
                sessions[TradingSession.PRE_MARKET] = (pre_start, pre_end)
            
            # Regular session
            regular_start = base_dt.replace(
                hour=self.market_hours.regular_open.hour,
                minute=self.market_hours.regular_open.minute
            )
            
            # Handle early close
            if is_early_close:
                regular_end = base_dt.replace(hour=13, minute=0)  # 1:00 PM ET
            else:
                regular_end = base_dt.replace(
                    hour=self.market_hours.regular_close.hour,
                    minute=self.market_hours.regular_close.minute
                )
            
            sessions[TradingSession.REGULAR] = (regular_start, regular_end)
            
            # After-hours session (only if not early close)
            if self.market_hours.after_hours_close and not is_early_close:
                after_start = regular_end
                after_end = base_dt.replace(
                    hour=self.market_hours.after_hours_close.hour,
                    minute=self.market_hours.after_hours_close.minute
                )
                sessions[TradingSession.AFTER_HOURS] = (after_start, after_end)
        
        notes = []
        if is_weekend:
            notes.append("Weekend")
        if is_holiday:
            notes.append("Market Holiday")
        if is_early_close:
            notes.append("Early Close (1:00 PM)")
        
        return TradingDay(
            date=target_date,
            market_type=self.market_type,
            is_trading_day=is_trading_day,
            sessions=sessions,
            early_close=is_early_close,
            notes=", ".join(notes)
        )
    
    def get_next_trading_day(self, from_date: Optional[date] = None) -> TradingDay:
        """Get the next trading day"""
        if from_date is None:
            from_date = self.get_current_time().date()
        
        current_date = from_date + timedelta(days=1)
        
        # Look ahead up to 10 days to find next trading day
        for _ in range(10):
            trading_day = self.get_trading_day(current_date)
            if trading_day.is_trading_day:
                return trading_day
            current_date += timedelta(days=1)
        
        # Fallback - return the date even if not a trading day
        return self.get_trading_day(current_date)
    
    def get_previous_trading_day(self, from_date: Optional[date] = None) -> TradingDay:
        """Get the previous trading day"""
        if from_date is None:
            from_date = self.get_current_time().date()
        
        current_date = from_date - timedelta(days=1)
        
        # Look back up to 10 days to find previous trading day
        for _ in range(10):
            trading_day = self.get_trading_day(current_date)
            if trading_day.is_trading_day:
                return trading_day
            current_date -= timedelta(days=1)
        
        # Fallback
        return self.get_trading_day(current_date)
    
    def is_market_open(self, dt: Optional[datetime] = None) -> bool:
        """Check if market is currently open"""
        session = self.get_current_session(dt)
        return session in [TradingSession.PRE_MARKET, TradingSession.REGULAR, TradingSession.AFTER_HOURS]
    
    def is_regular_hours(self, dt: Optional[datetime] = None) -> bool:
        """Check if market is in regular trading hours"""
        return self.get_current_session(dt) == TradingSession.REGULAR
    
    def time_until_market_open(self, dt: Optional[datetime] = None) -> Optional[timedelta]:
        """Get time until next market open"""
        if dt is None:
            dt = self.get_current_time()
        
        if self.is_market_open(dt):
            return timedelta(0)  # Market is already open
        
        # Find next market open
        current_date = dt.date()
        
        # Check today first
        trading_day = self.get_trading_day(current_date)
        if trading_day.is_trading_day:
            # Check if pre-market opens later today
            if TradingSession.PRE_MARKET in trading_day.sessions:
                pre_market_start = trading_day.sessions[TradingSession.PRE_MARKET][0]
                if dt < pre_market_start:
                    return pre_market_start - dt
            
            # Check if regular market opens later today
            if TradingSession.REGULAR in trading_day.sessions:
                regular_start = trading_day.sessions[TradingSession.REGULAR][0]
                if dt < regular_start:
                    return regular_start - dt
        
        # Look for next trading day
        next_trading_day = self.get_next_trading_day(current_date)
        if TradingSession.PRE_MARKET in next_trading_day.sessions:
            next_open = next_trading_day.sessions[TradingSession.PRE_MARKET][0]
        elif TradingSession.REGULAR in next_trading_day.sessions:
            next_open = next_trading_day.sessions[TradingSession.REGULAR][0]
        else:
            return None  # No market open found
        
        return next_open - dt
    
    def time_until_market_close(self, dt: Optional[datetime] = None) -> Optional[timedelta]:
        """Get time until market close"""
        if dt is None:
            dt = self.get_current_time()
        
        if not self.is_market_open(dt):
            return None  # Market is not open
        
        trading_day = self.get_trading_day(dt.date())
        current_session = self.get_current_session(dt)
        
        if current_session in trading_day.sessions:
            session_end = trading_day.sessions[current_session][1]
            return session_end - dt
        
        return None
    
    def schedule_at_time(
        self,
        target_time: Union[str, time, datetime],
        callback: callable,
        name: str = "",
        data: Optional[Dict] = None
    ) -> str:
        """Schedule an event at a specific time"""
        event_id = f"time_{len(self.scheduled_events)}_{datetime.now().timestamp()}"
        
        # Parse target time
        if isinstance(target_time, str):
            # Parse "HH:MM" format
            hour, minute = map(int, target_time.split(':'))
            target_time = time(hour, minute)
        
        if isinstance(target_time, time):
            # Convert to datetime for today
            current_dt = self.get_current_time()
            target_dt = datetime.combine(current_dt.date(), target_time, tzinfo=self.timezone)
            
            # If time has passed today, schedule for tomorrow
            if target_dt <= current_dt:
                target_dt += timedelta(days=1)
        else:
            target_dt = target_time
            if target_dt.tzinfo != self.timezone:
                target_dt = target_dt.astimezone(self.timezone)
        
        event = {
            "id": event_id,
            "name": name or f"Scheduled at {target_dt}",
            "target_time": target_dt,
            "callback": callback,
            "data": data or {},
            "created_at": self.get_current_time()
        }
        
        self.scheduled_events.append(event)
        logger.info(f"Scheduled event '{name}' for {target_dt}")
        
        return event_id
    
    def schedule_at_market_open(
        self,
        callback: callable,
        session: TradingSession = TradingSession.REGULAR,
        name: str = "",
        data: Optional[Dict] = None
    ) -> str:
        """Schedule an event at market open"""
        current_dt = self.get_current_time()
        
        # Find next market open for the specified session
        trading_day = self.get_trading_day(current_dt.date())
        
        if trading_day.is_trading_day and session in trading_day.sessions:
            target_dt = trading_day.sessions[session][0]
            if target_dt > current_dt:
                # Today's market open
                return self.schedule_at_time(
                    target_dt, callback, 
                    name or f"Market open ({session.value})", 
                    data
                )
        
        # Next trading day
        next_trading_day = self.get_next_trading_day()
        if session in next_trading_day.sessions:
            target_dt = next_trading_day.sessions[session][0]
            return self.schedule_at_time(
                target_dt, callback,
                name or f"Market open ({session.value})",
                data
            )
        
        raise ValueError(f"Cannot schedule for session {session.value}")
    
    def schedule_at_market_close(
        self,
        callback: callable,
        session: TradingSession = TradingSession.REGULAR,
        name: str = "",
        data: Optional[Dict] = None
    ) -> str:
        """Schedule an event at market close"""
        current_dt = self.get_current_time()
        
        # Find market close for the specified session
        trading_day = self.get_trading_day(current_dt.date())
        
        if trading_day.is_trading_day and session in trading_day.sessions:
            target_dt = trading_day.sessions[session][1]
            if target_dt > current_dt:
                # Today's market close
                return self.schedule_at_time(
                    target_dt, callback,
                    name or f"Market close ({session.value})",
                    data
                )
        
        # Next trading day
        next_trading_day = self.get_next_trading_day()
        if session in next_trading_day.sessions:
            target_dt = next_trading_day.sessions[session][1]
            return self.schedule_at_time(
                target_dt, callback,
                name or f"Market close ({session.value})",
                data
            )
        
        raise ValueError(f"Cannot schedule for session {session.value}")
    
    def schedule_recurring(
        self,
        callback: callable,
        interval: timedelta,
        name: str = "",
        start_time: Optional[datetime] = None,
        end_time: Optional[datetime] = None,
        trading_days_only: bool = True,
        data: Optional[Dict] = None
    ) -> str:
        """Schedule a recurring event"""
        event_id = f"recurring_{len(self.recurring_events)}_{datetime.now().timestamp()}"
        
        if start_time is None:
            start_time = self.get_current_time()
        
        event = {
            "id": event_id,
            "name": name or f"Recurring every {interval}",
            "callback": callback,
            "interval": interval,
            "start_time": start_time,
            "end_time": end_time,
            "trading_days_only": trading_days_only,
            "data": data or {},
            "last_executed": None,
            "created_at": self.get_current_time()
        }
        
        self.recurring_events.append(event)
        logger.info(f"Scheduled recurring event '{name}' every {interval}")
        
        return event_id
    
    def check_scheduled_events(self) -> List[Dict]:
        """Check for events that should be executed now"""
        current_dt = self.get_current_time()
        ready_events = []
        
        # Check one-time scheduled events
        for event in self.scheduled_events[:]:  # Copy list to allow modification
            if current_dt >= event["target_time"]:
                ready_events.append(event)
                self.scheduled_events.remove(event)
        
        # Check recurring events
        for event in self.recurring_events:
            should_execute = False
            
            if event["last_executed"] is None:
                # First execution
                if current_dt >= event["start_time"]:
                    should_execute = True
            else:
                # Check if interval has passed
                next_execution = event["last_executed"] + event["interval"]
                if current_dt >= next_execution:
                    should_execute = True
            
            # Check end time
            if event["end_time"] and current_dt > event["end_time"]:
                should_execute = False
            
            # Check trading days only
            if event["trading_days_only"] and not self.get_trading_day(current_dt.date()).is_trading_day:
                should_execute = False
            
            if should_execute:
                event["last_executed"] = current_dt
                ready_events.append(event.copy())  # Copy to avoid modification issues
        
        return ready_events
    
    def cancel_event(self, event_id: str) -> bool:
        """Cancel a scheduled event"""
        # Check one-time events
        for event in self.scheduled_events[:]:
            if event["id"] == event_id:
                self.scheduled_events.remove(event)
                logger.info(f"Cancelled scheduled event: {event_id}")
                return True
        
        # Check recurring events
        for event in self.recurring_events[:]:
            if event["id"] == event_id:
                self.recurring_events.remove(event)
                logger.info(f"Cancelled recurring event: {event_id}")
                return True
        
        return False
    
    def get_market_summary(self, target_date: Optional[date] = None) -> Dict:
        """Get comprehensive market summary for a date"""
        if target_date is None:
            target_date = self.get_current_time().date()
        
        trading_day = self.get_trading_day(target_date)
        current_dt = self.get_current_time()
        current_session = self.get_current_session(current_dt) if target_date == current_dt.date() else None
        
        summary = {
            "date": target_date.isoformat(),
            "market_type": self.market_type.value,
            "is_trading_day": trading_day.is_trading_day,
            "early_close": trading_day.early_close,
            "notes": trading_day.notes,
            "sessions": {},
            "current_session": current_session.value if current_session else None,
            "market_open": self.is_market_open(current_dt) if target_date == current_dt.date() else None
        }
        
        # Add session details
        for session, (start_dt, end_dt) in trading_day.sessions.items():
            summary["sessions"][session.value] = {
                "start": start_dt.strftime("%H:%M"),
                "end": end_dt.strftime("%H:%M"),
                "duration_minutes": int((end_dt - start_dt).total_seconds() / 60)
            }
        
        # Add time calculations if it's today
        if target_date == current_dt.date():
            time_to_open = self.time_until_market_open(current_dt)
            time_to_close = self.time_until_market_close(current_dt)
            
            if time_to_open:
                summary["time_until_open"] = str(time_to_open)
            if time_to_close:
                summary["time_until_close"] = str(time_to_close)
        
        return summary
    
    def get_scheduled_events_summary(self) -> Dict:
        """Get summary of all scheduled events"""
        return {
            "one_time_events": len(self.scheduled_events),
            "recurring_events": len(self.recurring_events),
            "next_events": [
                {
                    "name": event["name"],
                    "target_time": event["target_time"].isoformat(),
                    "type": "one_time"
                }
                for event in sorted(self.scheduled_events, key=lambda x: x["target_time"])[:5]
            ],
            "recurring_events_list": [
                {
                    "name": event["name"],
                    "interval": str(event["interval"]),
                    "last_executed": event["last_executed"].isoformat() if event["last_executed"] else None
                }
                for event in self.recurring_events
            ]
        }

# ============================================================================
# Global Time Manager Instance
# ============================================================================

# Default time manager for US stock market
time_manager = TimeScheduler(MarketType.STOCK)
