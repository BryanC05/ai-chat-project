import React, { useState } from 'react';
import './App.css';

// This URL stays the same
const API_URL = "/api/chat";

function App() {
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!input.trim()) return;

    const userMessage = { sender: 'user', text: input };
    
    // Create the new, full message history
    const newMessages = [...messages, userMessage];

    // Update the UI immediately
    setMessages(newMessages);
    setInput('');
    setIsLoading(true);

    try {
      // 1. UI -> Go API
      const response = await fetch(API_URL, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        
        // --- THIS IS THE FIX ---
        // We now send the *entire* message history
        // under the "messages" key, which the Go code expects.
        body: JSON.stringify({ messages: newMessages }),
      });

      if (!response.ok) {
        throw new Error('Failed to get a response from the bot.');
      }

      // 2. Go API -> UI
      const data = await response.json();
      const botMessage = { sender: 'bot', text: data.reply };
      
      // Add the bot's reply to the message history
      setMessages((prev) => [...prev, botMessage]);

    } catch (error) {
      const errorMessage = { sender: 'bot', text: error.message };
      setMessages((prev) => [...prev, errorMessage]);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="App">
      <div className="chat-window">
        {messages.map((msg, index) => (
          <div key={index} className={`message ${msg.sender}`}>
            {msg.text}
          </div>
        ))}
        {isLoading && (
          <div className="message bot">...</div>
        )}
      </div>
      <form className="chat-form" onSubmit={handleSubmit}>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Type your message..."
        />
        <button type="submit">Send</button>
      </form>
    </div>
  );
}

export default App;