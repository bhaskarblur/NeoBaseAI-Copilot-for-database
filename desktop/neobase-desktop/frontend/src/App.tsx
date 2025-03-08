import { useEffect, useState } from 'react';
import { GetUser, Login, Logout } from '../wailsjs/go/main/App';
import './App.css';

function App() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<any>(null);
  const [message, setMessage] = useState('');

  useEffect(() => {
    checkAuth();
  }, []);

  const checkAuth = async () => {
    try {
      const response = await GetUser();
      if (response.success && response.data) {
        setIsAuthenticated(true);
        setUser(response.data);
      } else {
        setIsAuthenticated(false);
        setUser(null);
      }
    } catch (error) {
      console.error('Auth check failed:', error);
      setIsAuthenticated(false);
      setUser(null);
    }
  };

  const handleLogin = async () => {
    try {
      const response = await Login(username, password);
      if (response.success && response.data) {
        setIsAuthenticated(true);
        setUser(response.data.user);
        setMessage(`Welcome back, ${response.data.user.username}!`);
      } else {
        setMessage(response.message || 'Login failed');
      }
    } catch (error: any) {
      setMessage(error.message || 'Login failed');
    }
  };

  const handleLogout = async () => {
    try {
      const response = await Logout();
      if (response.success) {
        setIsAuthenticated(false);
        setUser(null);
        setMessage('Logged out successfully');
      } else {
        setMessage(response.message || 'Logout failed');
      }
    } catch (error: any) {
      setMessage(error.message || 'Logout failed');
    }
  };

  return (
    <div className="container">
      <h1>NeoBase Desktop</h1>
      
      {message && (
        <div className="message">
          {message}
        </div>
      )}

      {!isAuthenticated ? (
        <div className="auth-form">
          <h2>Login</h2>
          <div className="form-group">
            <label htmlFor="username">Username</label>
            <input
              type="text"
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label htmlFor="password">Password</label>
            <input
              type="password"
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <button onClick={handleLogin}>Login</button>
          </div>
      ) : (
        <div className="dashboard">
          <h2>Welcome, {user?.username}</h2>
          <p>You are now logged in to NeoBase Desktop.</p>
          <button onClick={handleLogout}>Logout</button>
        </div>
      )}
    </div>
  );
}

export default App;