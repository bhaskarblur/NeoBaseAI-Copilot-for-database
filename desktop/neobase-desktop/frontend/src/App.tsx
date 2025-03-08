import { useEffect, useState } from 'react';
import { GetUser, Login, Logout } from '../wailsjs/go/main/App';
import './App.css';
import neobaseLogo from '/neobase-logo.svg';

function App() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<any>(null);
  const [message, setMessage] = useState('');
  const [isSignUp, setIsSignUp] = useState(false);

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

  const toggleSignUp = () => {
    setIsSignUp(!isSignUp);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#fdf6e3]">
      {!isAuthenticated ? (
        <div className="neo-border bg-white p-8 w-full max-w-md mx-auto">
          <div className="flex flex-col items-center mb-6">
            <div className="flex items-center justify-center mb-4 w-full">
              <div className="flex items-center">
                <img src={neobaseLogo} alt="NeoBase Logo" className="w-10 h-10 mr-2" />
                <h1 className="text-3xl font-bold text-black m-0">NeoBase</h1>
              </div>
            </div>
            <p className="text-gray-600 text-center">
              {isSignUp ? 'Create your NeoBase account' : 'Welcome back to the NeoBase!'}
            </p>
          </div>

          {message && (
            <div className="message mb-4">
              {message}
            </div>
          )}

          <div className="space-y-4">
            <div className="relative">
              <div className="absolute inset-y-0 left-3 flex items-center pointer-events-none">
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
                  <circle cx="12" cy="7" r="4"></circle>
                </svg>
              </div>
              <input
                type="text"
                id="username"
                placeholder="Username"
                className="neo-input w-full pl-10"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>

            <div className="relative">
              <div className="absolute inset-y-0 left-3 flex items-center pointer-events-none">
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                  <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
                </svg>
              </div>
              <input
                type="password"
                id="password"
                placeholder="Password"
                className="neo-input w-full pl-10"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>

            <button 
              className="neo-button w-full py-3" 
              onClick={isSignUp ? handleLogin : handleLogin}
            >
              {isSignUp ? 'Sign Up' : 'Login'}
            </button>

            <button 
              className="neo-button-secondary w-full py-3" 
              onClick={toggleSignUp}
            >
              {isSignUp ? 'Switch to Login' : 'Switch to Sign Up'}
            </button>
          </div>
        </div>
      ) : (
        <div className="dashboard w-full max-w-4xl mx-auto">
          <h2 className="text-2xl font-bold mb-6">Welcome, {user?.username}</h2>
          <p className="mb-6">You are now logged in to NeoBase Desktop.</p>
          <button className="neo-button" onClick={handleLogout}>Logout</button>
        </div>
      )}
    </div>
  );
}

export default App;