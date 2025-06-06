import React from 'react';
import { AlertCircle, ChevronDown, Key } from 'lucide-react';
import { Connection } from '../../../types/chat';

// We'll define FormErrors type here since it's not exported from ConnectionModal
interface FormErrors {
  host?: string;
  port?: string;
  database?: string;
  username?: string;
  ssh_host?: string;
  ssh_port?: string;
  ssh_username?: string;
  ssh_private_key?: string;
  auth_database?: string;
}

interface SSHConnectionTabProps {
  formData: Connection;
  errors: FormErrors;
  touched: Record<string, boolean>;
  handleChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void;
  handleBlur: (e: React.FocusEvent<HTMLInputElement>) => void;
  validateField: (name: string, value: Connection) => string | undefined;
  mongoUriSshInputRef: React.RefObject<HTMLInputElement>;
  onMongoUriChange?: (uri: string) => void;
}

const SSHConnectionTab: React.FC<SSHConnectionTabProps> = ({
  formData,
  errors,
  touched,
  handleChange,
  handleBlur,
  validateField,
  mongoUriSshInputRef,
  onMongoUriChange
}) => {
  // Custom blur handler that validates the field using the passed validateField function
  const handleFieldBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    const { name } = e.target;
    const error = validateField(name, formData);
    if (error) {
      // Error will be handled by the parent component via the validateField callback
      // The parent component updates the errors state
      console.log(`Validation error for ${name}: ${error}`);
    }
    // Call the parent's handleBlur to update touched state
    handleBlur(e);
  };

  return (
    <>
      {/* Coming Soon Overlay */}
      <div className="relative">
        <div className="absolute inset-0 bg-white/60 rounded-lg flex items-center justify-center z-10 -bottom-2 -right-2">
          <div className="absolute top-20 bg-white px-6 py-4 rounded-lg max-w-md text-center">
            <div className="flex items-center justify-center">
              <Key className="w-10 h-10 text-emerald-500 mb-2" />
            </div>
            <h3 className="text-xl font-bold mb-2">SSH Tunnel Coming Soon</h3>
            <p className="text-gray-700">
              SSH tunneling is currently under development. You can configure the settings below, 
              but the connection will use direct database connection for now.
            </p>
          </div>
        </div>

        <div className="space-y-6 opacity-70">
          {/* SSH Connection Section */}
          <div className="p-4 border-2 border-dashed border-gray-300 rounded-lg mb-6">
            <h3 className="font-bold text-lg mb-3">SSH Tunnel Configuration</h3>
            
            <div className="mb-4">
              <label className="block font-medium mb-1">SSH Host</label>
              <p className="text-gray-600 text-xs mb-1">Hostname or IP address of your SSH server</p>
              <input
                type="text"
                name="ssh_host"
                value={formData.ssh_host || ''}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.ssh_host && touched.ssh_host ? 'border-red-500' : ''}`}
                placeholder="e.g. ssh.example.com, 192.168.1.10"
              />
              {errors.ssh_host && touched.ssh_host && (
                <p className="text-red-500 text-xs mt-1">{errors.ssh_host}</p>
              )}
            </div>
            
            <div className="mb-4">
              <label className="block font-medium mb-1">SSH Port</label>
              <p className="text-gray-600 text-xs mb-1">Port for SSH connection (usually 22)</p>
              <input
                type="text"
                name="ssh_port"
                value={formData.ssh_port || '22'}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.ssh_port && touched.ssh_port ? 'border-red-500' : ''}`}
                placeholder="22"
              />
              {errors.ssh_port && touched.ssh_port && (
                <p className="text-red-500 text-xs mt-1">{errors.ssh_port}</p>
              )}
            </div>
            
            <div className="mb-4">
              <label className="block font-medium mb-1">SSH Username</label>
              <p className="text-gray-600 text-xs mb-1">Username for SSH authentication</p>
              <input
                type="text"
                name="ssh_username"
                value={formData.ssh_username || ''}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.ssh_username && touched.ssh_username ? 'border-red-500' : ''}`}
                placeholder="e.g. ubuntu, ec2-user"
              />
              {errors.ssh_username && touched.ssh_username && (
                <p className="text-red-500 text-xs mt-1">{errors.ssh_username}</p>
              )}
            </div>
            
            <div className="mb-4">
              <label className="block font-medium mb-1">SSH Private Key</label>
              <p className="text-gray-600 text-xs mb-1">Your SSH private key (OpenSSH format)</p>
              <textarea
                name="ssh_private_key"
                value={formData.ssh_private_key || ''}
                onChange={(e) => {
                  const mockEvent = {
                    target: {
                      name: 'ssh_private_key',
                      value: e.target.value
                    }
                  } as React.ChangeEvent<HTMLInputElement>;
                  handleChange(mockEvent);
                }}
                onBlur={(e) => {
                  const mockEvent = {
                    target: {
                      name: 'ssh_private_key'
                    }
                  } as React.FocusEvent<HTMLInputElement>;
                  handleBlur(mockEvent);
                  // Validate the field
                  validateField('ssh_private_key', formData);
                }}
                className={`neo-input w-full font-mono text-sm ${errors.ssh_private_key && touched.ssh_private_key ? 'border-red-500' : ''}`}
                placeholder="-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----"
                rows={5}
              />
              {errors.ssh_private_key && touched.ssh_private_key && (
                <p className="text-red-500 text-xs mt-1">{errors.ssh_private_key}</p>
              )}
            </div>
            
            <div className="mb-4">
              <label className="block font-medium mb-1">SSH Passphrase (Optional)</label>
              <p className="text-gray-600 text-xs mb-1">If your private key is encrypted with a passphrase</p>
              <input
                type="password"
                name="ssh_passphrase"
                value={formData.ssh_passphrase || ''}
                onChange={handleChange}
                className="neo-input w-full"
                placeholder="Leave empty if your key doesn't have a passphrase"
              />
            </div>
          </div>

          {/* Database Settings Section */}
          <div>
            <h3 className="font-bold text-lg mb-3">Database Settings</h3>
            
            {/* Database Type */}
            <div className="mb-6">
              <label className="block font-bold mb-2">Database Type</label>
              <p className="text-gray-600 text-sm mb-2">Select your database system</p>
              <div className="relative">
                <select
                  name="type"
                  value={formData.type}
                  onChange={handleChange}
                  className="neo-input w-full appearance-none pr-12"
                >
                  {[
                    { value: 'postgresql', label: 'PostgreSQL' },
                    { value: 'yugabytedb', label: 'YugabyteDB' },
                    { value: 'mysql', label: 'MySQL' },
                    { value: 'clickhouse', label: 'ClickHouse' },
                    { value: 'mongodb', label: 'MongoDB' },
                    { value: 'cassandra', label: 'Cassandra (Coming Soon)' },
                    { value: 'redis', label: 'Redis (Coming Soon)' },
                    { value: 'neo4j', label: 'Neo4J (Coming Soon)' }
                  ].map(option => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                <div className="absolute inset-y-0 right-0 flex items-center pr-4 pointer-events-none">
                  <ChevronDown className="w-5 h-5 text-gray-400" />
                </div>
              </div>
            </div>

            {/* MongoDB Connection URI Field - Only show when MongoDB is selected */}
            {formData.type === 'mongodb' && (
              <div className="mb-6">
                <label className="block font-bold mb-2">MongoDB Connection URI</label>
                <p className="text-gray-600 text-sm mb-2">Paste your MongoDB connection string to auto-fill fields</p>
                <input
                  type="text"
                  name="mongo_uri_ssh"
                  ref={mongoUriSshInputRef}
                  className="neo-input w-full"
                  placeholder="mongodb://username:password@host:port/database or mongodb+srv://username:password@host/database"
                  onChange={(e) => {
                    const uri = e.target.value;
                    // Save the URI value through the callback
                    if (onMongoUriChange) {
                      onMongoUriChange(uri);
                    }
                    try {
                      // Better parsing logic for MongoDB URIs that can handle special characters in credentials
                      const srvFormat = uri.startsWith('mongodb+srv://');
                      
                      // Extract the protocol and the rest
                      const protocolMatch = uri.match(/^(mongodb(?:\+srv)?:\/\/)(.*)/);
                      if (!protocolMatch) {
                        console.log("Invalid MongoDB URI format: Missing protocol");
                        return;
                      }
                      
                      const [, , remainder] = protocolMatch;
                      
                      // Check if credentials are provided (look for @ after the protocol)
                      const hasCredentials = remainder.includes('@');
                      let username = '';
                      let password = '';
                      let hostPart = remainder;
                      
                      if (hasCredentials) {
                        // Find the last @ which separates credentials from host
                        const lastAtIndex = remainder.lastIndexOf('@');
                        const credentialsPart = remainder.substring(0, lastAtIndex);
                        hostPart = remainder.substring(lastAtIndex + 1);
                        
                        // Find the first : which separates username from password
                        const firstColonIndex = credentialsPart.indexOf(':');
                        if (firstColonIndex !== -1) {
                          username = credentialsPart.substring(0, firstColonIndex);
                          password = credentialsPart.substring(firstColonIndex + 1);
                          
                          // Handle URL encoded characters in username and password
                          try {
                            username = decodeURIComponent(username);
                            password = decodeURIComponent(password);
                          } catch (e) {
                            console.log("Could not decode URI components:", e);
                          }
                        } else {
                          username = credentialsPart;
                          try {
                            username = decodeURIComponent(username);
                          } catch (e) {
                            console.log("Could not decode username:", e);
                          }
                        }
                      }
                      
                      // Parse host, port and database
                      let host = '';
                      let port = srvFormat ? '27017' : ''; // Default for SRV format
                      let database = 'test'; // Default database name
                      let authDatabase = 'admin'; // Default auth database
                      
                      // Check if there's a / after the host[:port] part
                      const pathIndex = hostPart.indexOf('/');
                      if (pathIndex !== -1) {
                        const hostPortPart = hostPart.substring(0, pathIndex);
                        const pathPart = hostPart.substring(pathIndex + 1);
                        
                        // Extract database name and query parameters
                        const queryIndex = pathPart.indexOf('?');
                        if (queryIndex !== -1) {
                          database = pathPart.substring(0, queryIndex);
                          const queryParams = new URLSearchParams(pathPart.substring(queryIndex + 1));
                          if (queryParams.has('authSource')) {
                            authDatabase = queryParams.get('authSource') || 'admin';
                          }
                        } else {
                          database = pathPart;
                        }
                        
                        // Parse host and port
                        const portIndex = hostPortPart.indexOf(':');
                        if (portIndex !== -1) {
                          host = hostPortPart.substring(0, portIndex);
                          port = hostPortPart.substring(portIndex + 1);
                        } else {
                          host = hostPortPart;
                        }
                      } else {
                        // No database specified in the URI
                        const portIndex = hostPart.indexOf(':');
                        if (portIndex !== -1) {
                          host = hostPart.substring(0, portIndex);
                          port = hostPart.substring(portIndex + 1);
                        } else {
                          host = hostPart;
                        }
                      }
                      
                      if (host) {
                        console.log("MongoDB URI parsed successfully", { username, host, port, database, authDatabase });
                        
                        // Update formData through parent component
                        const newFormData = {
                          ...formData,
                          host: host,
                          port: port || (srvFormat ? '27017' : formData.port),
                          database: database || 'test',
                          auth_database: authDatabase,
                          username: username || formData.username,
                          password: password || formData.password
                        };
                        
                        // Trigger handleChange with each field
                        const mockEvent = (name: string, value: string) => ({
                          target: { name, value }
                        }) as React.ChangeEvent<HTMLInputElement>;
                        
                        handleChange(mockEvent('host', newFormData.host));
                        handleChange(mockEvent('port', newFormData.port));
                        handleChange(mockEvent('database', newFormData.database));
                        handleChange(mockEvent('auth_database', newFormData.auth_database));
                        handleChange(mockEvent('username', newFormData.username));
                        if (password) {
                          handleChange(mockEvent('password', password));
                        }
                      } else {
                        console.log("MongoDB URI parsing failed: could not extract host");
                      }
                    } catch (err) {
                      // Invalid URI format, just continue
                      console.log("Invalid MongoDB URI format", err);
                    }
                  }}
                />
                <p className="text-gray-500 text-xs mt-2">
                  Connection URI will be used to auto-fill the fields below. Both standard and Atlas SRV formats supported.
                </p>
              </div>
            )}
            
            {/* Host */}
            <div className="mb-6">
              <label className="block font-bold mb-2">Database Host</label>
              <p className="text-gray-600 text-sm mb-2">Hostname or IP address of your database</p>
              <input
                type="text"
                name="host"
                value={formData.host}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.host && touched.host ? 'border-red-500' : ''}`}
                placeholder="e.g. localhost, 127.0.0.1"
              />
              {errors.host && touched.host && (
                <div className="flex items-center gap-1 mt-1 text-red-500 text-sm">
                  <AlertCircle className="w-4 h-4" />
                  <span>{errors.host}</span>
                </div>
              )}
            </div>
            
            {/* Port */}
            <div className="mb-6">
              <label className="block font-bold mb-2">Database Port</label>
              <p className="text-gray-600 text-sm mb-2">Port your database is listening on</p>
              <input
                type="text"
                name="port"
                value={formData.port}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.port && touched.port ? 'border-red-500' : ''}`}
                placeholder="e.g. 5432, 3306, 27017"
              />
              {errors.port && touched.port && (
                <div className="flex items-center gap-1 mt-1 text-red-500 text-sm">
                  <AlertCircle className="w-4 h-4" />
                  <span>{errors.port}</span>
                </div>
              )}
            </div>
            
            {/* Database */}
            <div className="mb-6">
              <label className="block font-bold mb-2">Database Name</label>
              <p className="text-gray-600 text-sm mb-2">Name of the database to connect to</p>
              <input
                type="text"
                name="database"
                value={formData.database}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.database && touched.database ? 'border-red-500' : ''}`}
                placeholder="e.g. postgres, mydatabase"
              />
              {errors.database && touched.database && (
                <div className="flex items-center gap-1 mt-1 text-red-500 text-sm">
                  <AlertCircle className="w-4 h-4" />
                  <span>{errors.database}</span>
                </div>
              )}
            </div>
            
            {/* Username */}
            <div className="mb-6">
              <label className="block font-bold mb-2">Database Username</label>
              <p className="text-gray-600 text-sm mb-2">Username for database authentication</p>
              <input
                type="text"
                name="username"
                value={formData.username}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.username && touched.username ? 'border-red-500' : ''}`}
                placeholder="e.g. postgres, root, admin"
              />
              {errors.username && touched.username && (
                <div className="flex items-center gap-1 mt-1 text-red-500 text-sm">
                  <AlertCircle className="w-4 h-4" />
                  <span>{errors.username}</span>
                </div>
              )}
            </div>
            
            {/* Password */}
            <div className="mb-6">
              <label className="block font-bold mb-2">Database Password</label>
              <p className="text-gray-600 text-sm mb-2">Password for database authentication</p>
              <input
                type="password"
                name="password"
                value={formData.password || ''}
                onChange={handleChange}
                className="neo-input w-full"
                placeholder="Enter your database password"
              />
              <p className="text-gray-500 text-xs mt-2">Leave blank if the database has no password, but it's recommended to set a password for the database user</p>
            </div>

            {/* MongoDB Authentication Database Field - Only show when MongoDB is selected */}
            {formData.type === 'mongodb' && (
              <div className="mb-6">
                <label className="block font-bold mb-2">Authentication Database</label>
                <p className="text-gray-600 text-sm mb-2">The database to authenticate against (usually 'admin' for MongoDB)</p>
                <input
                  type="text"
                  name="auth_database"
                  value={formData.auth_database || 'admin'}
                  onChange={handleChange}
                  onBlur={handleFieldBlur}
                  className={`neo-input w-full ${errors.auth_database && touched.auth_database ? 'border-red-500' : ''}`}
                  placeholder="e.g., admin"
                />
                {errors.auth_database && touched.auth_database && (
                  <div className="flex items-center gap-1 mt-1 text-red-500 text-sm">
                    <AlertCircle className="w-4 h-4" />
                    <span>{errors.auth_database}</span>
                  </div>
                )}
                <p className="text-gray-500 text-xs mt-2">
                  This is the database where your user credentials are stored. For MongoDB Atlas, this is usually 'admin'.
                </p>
              </div>
            )}

          </div>
        </div>
      </div>
    </>
  );
};

export default SSHConnectionTab; 