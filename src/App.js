import React, { useState } from 'react';
import ReactMarkdown from 'react-markdown'; // <-- 1. IMPORT THE LIBRARY
import './App.css';

const API_URL = "/api/chat";

function App() {
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!input.trim()) return;

    const userMessage = { sender: 'user', text: input };
    const newMessages = [...messages, userMessage];

    setMessages(newMessages);
    setInput('');
    setIsLoading(true);

    try {
      const response = await fetch(API_URL, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ messages: newMessages }),
      });

      if (!response.ok) {
        throw new Error('Failed to get a response from the bot.');
      }

      const data = await response.json();
      const botMessage = { sender: 'bot', text: data.reply };
      
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
          // --- 2. THIS IS THE KEY CHANGE ---
          // We check if the sender is the 'bot'.
          // If yes, we render the text using <ReactMarkdown>.
          // If no (it's the user), we just show the plain text.
          <div key={index} className={`message ${msg.sender}`}>
            {msg.sender === 'user' ? (
              msg.text
            ) : (
              <ReactMarkdown>{msg.text}</ReactMarkdown>
            )}
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