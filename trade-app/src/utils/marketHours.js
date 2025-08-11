import { DateTime } from "luxon";

/**
 * Market Hours Utility - Centralized market hours logic
 * Reusable across the application for consistent market status determination
 */
export class MarketHoursUtil {
  /**
   * Check if current time is within market hours
   * Market hours: 9:30 AM - 4:00 PM Eastern Time, Monday-Friday
   */
  static isMarketOpen() {
    const now = DateTime.now().setZone("America/New_York");
    const day = now.weekday; // 1=Monday, 7=Sunday
    const hour = now.hour;
    const minute = now.minute;
    
    // US market open: Mon-Fri, 9:30am-4:00pm ET
    const isWeekday = day >= 1 && day <= 5;
    const isOpen = isWeekday && (
      (hour > 9 || (hour === 9 && minute >= 30)) &&
      (hour < 16)
    );
    
    return isOpen;
  }
  
  /**
   * Get current Eastern Time for logging
   */
  static getCurrentEasternTime() {
    return DateTime.now().setZone("America/New_York");
  }
  
  /**
   * Get market status string for logging and UI
   */
  static getMarketStatus() {
    const isOpen = this.isMarketOpen();
    const easternTime = this.getCurrentEasternTime();
    const timeString = easternTime.toLocaleString(DateTime.TIME_WITH_SECONDS);
    
    return {
      isOpen,
      timeString,
      status: isOpen ? 'Market Open' : 'Market Closed'
    };
  }
  
  /**
   * Get market status text (compatible with existing code)
   */
  static getMarketStatusText() {
    return this.isMarketOpen() ? "Market Open" : "Market Closed";
  }
}

export default MarketHoursUtil;
