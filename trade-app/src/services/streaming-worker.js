// streaming-worker.js

let socket = null;
let reconnectInterval = 5000; // 5 seconds
let subscriptions = new Set();
let messageQueue = []; // Queue for messages sent before connection is open

function connect(url) {
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    console.log("WebSocket is already connected or connecting.");
    return;
  }

  socket = new WebSocket(url);

  socket.onopen = () => {
    console.log("WebSocket connection established in worker.");
    postMessage({ type: 'status', message: 'connected' });
    
    // Process any queued messages
    while (messageQueue.length > 0) {
      const queuedMessage = messageQueue.shift();
      socket.send(queuedMessage);
    }

    // Resubscribe to any existing symbols
    if (subscriptions.size > 0) {
      socket.send(JSON.stringify({ action: 'subscribe', symbols: Array.from(subscriptions) }));
    }
  };

  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      postMessage({ type: 'data', payload: data });
    } catch (error) {
      console.error("Error parsing message in worker:", error);
    }
  };

  socket.onclose = () => {
    console.log("WebSocket connection closed in worker. Attempting to reconnect...");
    postMessage({ type: 'status', message: 'disconnected' });
    setTimeout(() => connect(url), reconnectInterval);
  };

  socket.onerror = (error) => {
    console.error("WebSocket error in worker:", error);
    socket.close(); // This will trigger the onclose event and reconnection logic
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

self.onmessage = (event) => {
  const { command, url, symbols, message } = event.data;

  switch (command) {
    case 'connect':
      connect(url);
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
    case 'subscribe_persistent':
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify(event.data));
      } else {
        console.error("Cannot send persistent subscription, WebSocket is not open.");
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
    default:
      console.error(`Unknown command in worker: ${command}`);
  }
};
