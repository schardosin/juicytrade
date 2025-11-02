// streaming-worker.js

let socket = null;
let reconnectInterval = 5000; // 5 seconds
let subscriptions = new Set();
let messageQueue = []; // Queue for messages sent before connection is open

// Enhanced connection monitoring
let lastDataReceived = Date.now();
let heartbeatInterval = null;
let staleConnectionTimeout = null;
let reconnectAttempts = 0;
let maxReconnectAttempts = 10;
let isManuallyDisconnected = false;

// Connection quality monitoring
const CONNECTION_STATES = {
  DISCONNECTED: 'disconnected',
  CONNECTING: 'connecting', 
  CONNECTED: 'connected',
  STALE: 'stale',
  RECOVERING: 'recovering'
};

let connectionState = CONNECTION_STATES.DISCONNECTED;

function connect(url) {
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    console.log("WebSocket is already connected or connecting.");
    return;
  }

  // Clear any existing timers
  clearConnectionTimers();
  
  // Update connection state
  updateConnectionState(CONNECTION_STATES.CONNECTING);
  isManuallyDisconnected = false;

  socket = new WebSocket(url);

  socket.onopen = () => {
    reconnectAttempts = 0; // Reset on successful connection
    lastDataReceived = Date.now();
    
    updateConnectionState(CONNECTION_STATES.CONNECTED);
    
    // Start monitoring for stale connections
    startStaleConnectionMonitoring();
    
    // Process any queued messages
    while (messageQueue.length > 0) {
      const queuedMessage = messageQueue.shift();
      socket.send(queuedMessage);
    }

    // Resubscribe to any existing symbols
    if (subscriptions.size > 0) {
      console.log(`🔄 Resubscribing to ${subscriptions.size} symbols after reconnection`);
      socket.send(JSON.stringify({ action: 'subscribe', symbols: Array.from(subscriptions) }));
    }
  };

  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      
      // Update last data received timestamp for any meaningful data
      if (data.type === 'price_update' || data.type === 'greeks_update') {
        lastDataReceived = Date.now();
        
        // If we were in stale state, we're now healthy again
        if (connectionState === CONNECTION_STATES.STALE) {
          console.log("✅ Connection recovered from stale state");
          updateConnectionState(CONNECTION_STATES.CONNECTED);
        }
      }
      
      postMessage({ type: 'data', payload: data });
    } catch (error) {
      console.error("Error parsing message in worker:", error);
    }
  };

  socket.onclose = (event) => {
    console.log(`🔌 WebSocket connection closed (code: ${event.code}, reason: ${event.reason})`);
    clearConnectionTimers();
    
    // Check if this is an authentication-related closure
    const isAuthFailure = event.code === 1008 || event.code === 1011 || event.code === 403 || event.code === 4001;
    
    if (!isManuallyDisconnected) {
      updateConnectionState(CONNECTION_STATES.DISCONNECTED);
      
      if (isAuthFailure) {
        console.log("🔒 WebSocket closed due to authentication failure - stopping reconnection attempts");
        // Set manual disconnect flag to prevent any further reconnection attempts
        isManuallyDisconnected = true;
        // Don't attempt to reconnect on authentication failures
        postMessage({ 
          type: 'status', 
          message: 'auth_failed',
          detail: { code: event.code, reason: event.reason }
        });
      } else {
        scheduleReconnect(url);
      }
    }
  };

  socket.onerror = (error) => {
    console.error("❌ WebSocket error in worker:", error);
    clearConnectionTimers();
    updateConnectionState(CONNECTION_STATES.DISCONNECTED);
    
    if (socket) {
      socket.close(); // This will trigger the onclose event and reconnection logic
    }
  };
}

function subscribe(symbols) {
  symbols.forEach(symbol => subscriptions.add(symbol));
  const message = JSON.stringify({ action: 'subscribe', symbols });
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(message);
  } else {
    messageQueue.push(message);
  }
}

function unsubscribe(symbols) {
  symbols.forEach(symbol => subscriptions.delete(symbol));
  const message = JSON.stringify({ action: 'unsubscribe', symbols });
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(message);
  } else {
    messageQueue.push(message);
  }
}

// Helper functions for connection monitoring
function updateConnectionState(newState) {
  if (connectionState !== newState) {
    connectionState = newState;
    postMessage({ type: 'status', message: newState });
  }
}

function clearConnectionTimers() {
  if (heartbeatInterval) {
    clearInterval(heartbeatInterval);
    heartbeatInterval = null;
  }
  if (staleConnectionTimeout) {
    clearTimeout(staleConnectionTimeout);
    staleConnectionTimeout = null;
  }
}

function startStaleConnectionMonitoring() {
  // Clear any existing monitoring
  clearConnectionTimers();
  
  // Monitor for stale connections every 30 seconds
  heartbeatInterval = setInterval(() => {
    const timeSinceLastData = Date.now() - lastDataReceived;
    const staleThreshold = 60000; // 1 minute without data = stale
    
    if (timeSinceLastData > staleThreshold && connectionState === CONNECTION_STATES.CONNECTED) {
      console.warn(`⚠️ Connection appears stale (no data for ${Math.round(timeSinceLastData/1000)}s)`);
      updateConnectionState(CONNECTION_STATES.STALE);
      
      // Trigger recovery after being stale for 30 seconds
      staleConnectionTimeout = setTimeout(() => {
        if (connectionState === CONNECTION_STATES.STALE) {
          console.log("🚑 Triggering recovery for stale connection");
          triggerRecovery();
        }
      }, 30000);
    }
  }, 30000);
}

function triggerRecovery() {
  console.log("🔄 Starting connection recovery");
  updateConnectionState(CONNECTION_STATES.RECOVERING);
  
  // Force close current connection to trigger reconnection
  if (socket) {
    isManuallyDisconnected = true; // Prevent normal reconnection logic
    socket.close();
    isManuallyDisconnected = false;
  }
  
  // Notify main thread about recovery
  postMessage({ 
    type: 'recovery', 
    message: 'stale_connection_recovery',
    detail: {
      lastDataReceived: lastDataReceived,
      timeSinceLastData: Date.now() - lastDataReceived
    }
  });
}

function scheduleReconnect(url) {
  // Additional safety check - don't reconnect if manually disconnected
  if (isManuallyDisconnected) {
    console.log("🚫 Reconnection attempt blocked - manual disconnect in effect");
    return;
  }

  if (reconnectAttempts >= maxReconnectAttempts) {
    console.error(`❌ Max reconnection attempts (${maxReconnectAttempts}) reached`);
    updateConnectionState(CONNECTION_STATES.DISCONNECTED);
    return;
  }
  
  reconnectAttempts++;
  const delay = Math.min(reconnectInterval * Math.pow(2, reconnectAttempts - 1), 30000); // Exponential backoff, max 30s
  
  console.log(`🔄 Scheduling reconnection attempt ${reconnectAttempts}/${maxReconnectAttempts} in ${delay}ms`);
  
  setTimeout(() => {
    // Double-check before attempting reconnection
    if (!isManuallyDisconnected) {
      connect(url);
    } else {
      console.log("🚫 Reconnection attempt aborted - manual disconnect detected");
    }
  }, delay);
}

function disconnect() {
  console.log("🔌 Manually disconnecting WebSocket - preventing all reconnection");
  
  // CRITICAL: Set this FIRST to prevent any reconnection attempts
  isManuallyDisconnected = true;
  
  // Clear all timers and intervals immediately
  clearConnectionTimers();
  
  // Clear any pending reconnection timeouts
  if (typeof reconnectTimeoutId !== 'undefined' && reconnectTimeoutId) {
    clearTimeout(reconnectTimeoutId);
    reconnectTimeoutId = null;
  }
  
  // Close WebSocket connection immediately
  if (socket) {
    try {
      socket.close(1000, "Manual disconnect - worker terminating");
      console.log("🔌 WebSocket closed with normal closure code");
    } catch (error) {
      console.warn("⚠️ Error closing WebSocket:", error);
    }
    socket = null;
  }
  
  // Clear all subscriptions and queued messages
  subscriptions.clear();
  messageQueue.length = 0;
  
  // Reset connection state
  reconnectAttempts = 0;
  lastDataReceived = Date.now();
  
  updateConnectionState(CONNECTION_STATES.DISCONNECTED);
  
  console.log("✅ Worker disconnect complete - all cleanup finished");
}

self.onmessage = (event) => {
  const { command, url, symbols, message } = event.data;

  switch (command) {
    case 'connect':
      connect(url);
      break;
    case 'disconnect':
      disconnect();
      break;
    case 'subscribe':
      subscribe(symbols);
      break;
    case 'unsubscribe':
      unsubscribe(symbols);
      break;
    case 'message':
      const messageToSend = JSON.stringify(message);
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(messageToSend);
      } else {
        messageQueue.push(messageToSend);
      }
      break;
    case 'subscribe_replace_all':
      const { stock_symbols, option_symbols } = event.data;
      
      // Update the worker's internal subscription set
      subscriptions = new Set([...stock_symbols, ...option_symbols]);

      const replaceMessage = JSON.stringify({
        type: 'subscribe_smart_replace_all',
        stock_symbols,
        option_symbols,
        all_symbols: [...stock_symbols, ...option_symbols]
      });

      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(replaceMessage);
      } else {
        console.log('[streaming-worker] WebSocket not open. Queuing message.');
        messageQueue.push(replaceMessage);
      }
      break;
    case 'force_recovery':
      console.log("🚑 Force recovery requested");
      triggerRecovery();
      break;
    case 'keepalive':
      const keepaliveMessage = JSON.stringify({
        type: 'keepalive',
        symbols: event.data.symbols
      });
      
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(keepaliveMessage);
      } else {
        console.log('[streaming-worker] WebSocket not open. Queuing keepalive message.');
        messageQueue.push(keepaliveMessage);
      }
      break;
    case 'get_status':
      postMessage({
        type: 'status_info',
        message: {
          connectionState: connectionState,
          lastDataReceived: lastDataReceived,
          timeSinceLastData: Date.now() - lastDataReceived,
          reconnectAttempts: reconnectAttempts,
          subscriptions: Array.from(subscriptions)
        }
      });
      break;
    default:
      console.error(`Unknown command in worker: ${command}`);
  }
};
