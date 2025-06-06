import React from 'react';
import { AlertCircle, ChevronDown } from 'lucide-react';
import { Connection } from '../../../types/chat';

// Define FormErrors interface locally instead of importing it
interface FormErrors {
  host?: string;
  port?: string;
  database?: string;
  username?: string;
  auth_database?: string;
  ssl_cert_url?: string;
  ssl_key_url?: string;
  ssl_root_cert_url?: string;
  ssh_host?: string;
  ssh_port?: string;
  ssh_username?: string;
  ssh_private_key?: string;
}

interface BasicConnectionTabProps {
  formData: Connection;
  errors: FormErrors;
  touched: Record<string, boolean>;
  handleChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => void;
  handleBlur: (e: React.FocusEvent<HTMLInputElement>) => void;
  validateField: (name: string, value: Connection) => string | undefined;
  mongoUriInputRef: React.RefObject<HTMLInputElement>;
  onMongoUriChange?: (uri: string) => void;
}

const BasicConnectionTab: React.FC<BasicConnectionTabProps> = ({
  formData,
  errors,
  touched,
  handleChange,
  handleBlur,
  validateField,
  mongoUriInputRef,
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
      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">Database Type</label>
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
          <label className="block font-bold mb-2 text-lg">MongoDB Connection URI</label>
          <p className="text-gray-600 text-sm mb-2">Paste your MongoDB connection string to auto-fill fields</p>
          <input
            type="text"
            name="mongo_uri"
            ref={mongoUriInputRef}
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
                
                const [, protocol, remainder] = protocolMatch;
                
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

      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">Host</label>
        <p className="text-gray-600 text-sm mb-2">The hostname or IP address of your database server</p>
        <input
          type="text"
          name="host"
          value={formData.host}
          onChange={handleChange}
          onBlur={handleFieldBlur}
          className={`neo-input w-full ${errors.host && touched.host ? 'border-neo-error' : ''}`}
          placeholder="e.g., localhost, db.example.com, 192.168.1.1"
          required
        />
        {errors.host && touched.host && (
          <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
            <AlertCircle className="w-4 h-4" />
            <span>{errors.host}</span>
          </div>
        )}
      </div>

      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">Port</label>
        <p className="text-gray-600 text-sm mb-2">The port number your database is listening on</p>
        <input
          type="text"
          name="port"
          value={formData.port}
          onChange={handleChange}
          onBlur={handleFieldBlur}
          className={`neo-input w-full ${errors.port && touched.port ? 'border-neo-error' : ''}`}
          placeholder="e.g., 5432 (PostgreSQL), 3306 (MySQL), 27017 (MongoDB)"
        />
        {errors.port && touched.port && (
          <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
            <AlertCircle className="w-4 h-4" />
            <span>{errors.port}</span>
          </div>
        )}
      </div>

      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">Database Name</label>
        <p className="text-gray-600 text-sm mb-2">The name of the specific database to connect to</p>
        <input
          type="text"
          name="database"
          value={formData.database}
          onChange={handleChange}
          onBlur={handleFieldBlur}
          className={`neo-input w-full ${errors.database && touched.database ? 'border-neo-error' : ''}`}
          placeholder="e.g., myapp_production, users_db"
          required
        />
        {errors.database && touched.database && (
          <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
            <AlertCircle className="w-4 h-4" />
            <span>{errors.database}</span>
          </div>
        )}
      </div>

      {/* MongoDB Authentication Database Field - Only show when MongoDB is selected */}
      {formData.type === 'mongodb' && (
        <div className="mb-6">
          <label className="block font-bold mb-2 text-lg">Authentication Database</label>
          <p className="text-gray-600 text-sm mb-2">The database to authenticate against (usually 'admin' for MongoDB)</p>
          <input
            type="text"
            name="auth_database"
            value={formData.auth_database || 'admin'}
            onChange={handleChange}
            onBlur={handleFieldBlur}
            className={`neo-input w-full ${errors.auth_database && touched.auth_database ? 'border-neo-error' : ''}`}
            placeholder="e.g., admin"
          />
          {errors.auth_database && touched.auth_database && (
            <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
              <AlertCircle className="w-4 h-4" />
              <span>{errors.auth_database}</span>
            </div>
          )}
          <p className="text-gray-500 text-xs mt-2">
            This is the database where your user credentials are stored. For MongoDB Atlas, this is usually 'admin'.
          </p>
        </div>
      )}

      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">Username</label>
        <p className="text-gray-600 text-sm mb-2">Database user with appropriate permissions</p>
        <input
          type="text"
          name="username"
          value={formData.username}
          onChange={handleChange}
          onBlur={handleFieldBlur}
          className={`neo-input w-full ${errors.username && touched.username ? 'border-neo-error' : ''}`}
          placeholder="e.g., db_user, assistant"
          required
        />
        {errors.username && touched.username && (
          <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
            <AlertCircle className="w-4 h-4" />
            <span>{errors.username}</span>
          </div>
        )}
      </div>

      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">Password</label>
        <p className="text-gray-600 text-sm mb-2">Password for the database user</p>
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

      {/* Divider line */}
      <div className="border-t border-gray-200 my-6"></div>

      {/* SSL Toggle */}
      <div className="mb-6">
        <label className="block font-bold mb-2 text-lg">SSL/TLS Security</label>
        <p className="text-gray-600 text-sm mb-2">Enable secure connection to your database</p>
        <div className="flex items-center">
          <input
            type="checkbox"
            id="use_ssl"
            name="use_ssl"
            checked={formData.use_ssl || false}
            onChange={(e) => {
              const useSSL = e.target.checked;
              const mockEvent = {
                target: { 
                  name: 'use_ssl', 
                  value: useSSL 
                }
              } as unknown as React.ChangeEvent<HTMLInputElement>;
              handleChange(mockEvent);
              
              // Also update ssl_mode if turning off SSL
              if (!useSSL) {
                const sslModeEvent = {
                  target: { 
                    name: 'ssl_mode', 
                    value: 'disable' 
                  }
                } as unknown as React.ChangeEvent<HTMLSelectElement>;
                handleChange(sslModeEvent);
              }
            }}
            className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
          />
          <label htmlFor="use_ssl" className="ml-2 block text-sm font-medium text-gray-700">
            Use SSL/TLS encryption
          </label>
        </div>
      </div>

      {/* SSL Mode Selector - Only show when SSL is enabled */}
      {formData.use_ssl && (
        <div className="mb-6">
          <label className="block font-medium mb-2">SSL Mode</label>
          <div className="relative">
            <select
              name="ssl_mode"
              value={formData.ssl_mode || 'disable'}
              onChange={handleChange}
              className="neo-input w-full appearance-none pr-12"
            >
              <option value="disable">Disable - No SSL</option>
              <option value="require">Require - Encrypted only</option>
              <option value="verify-ca">Verify CA - Verify certificate authority</option>
              <option value="verify-full">Verify Full - Verify CA and hostname</option>
            </select>
            <div className="absolute inset-y-0 right-0 flex items-center pr-4 pointer-events-none">
              <ChevronDown className="w-5 h-5 text-gray-400" />
            </div>
          </div>
          <p className="text-gray-500 text-xs mt-2">
            {formData.ssl_mode === 'disable' && 'SSL will not be used.'}
            {formData.ssl_mode === 'require' && 'Connection must be encrypted, but certificates are not verified.'}
            {formData.ssl_mode === 'verify-ca' && 'Connection must be encrypted and the server certificate must be verified.'}
            {formData.ssl_mode === 'verify-full' && 'Connection must be encrypted and both the server certificate and hostname must be verified.'}
          </p>
        </div>
      )}

      {/* SSL Certificate Fields - Only show when SSL is enabled and mode requires verification */}
      {formData.use_ssl && (formData.ssl_mode === 'verify-ca' || formData.ssl_mode === 'verify-full') && (
        <div className="mb-6 p-4 border-dashed border-2 border-gray-200 rounded-md bg-gray-50">
          <h4 className="font-bold mb-3 text-md">SSL/TLS Certificate Configuration</h4>
          
          <div className="mb-4">
            <label className="block font-medium mb-1 text-sm">SSL Certificate URL</label>
            <p className="text-gray-600 text-xs mb-1">URL to your client certificate file (.pem or .crt)</p>
            <input
              type="text"
              name="ssl_cert_url"
              value={formData.ssl_cert_url || ''}
              onChange={handleChange}
              onBlur={handleFieldBlur}
              className={`neo-input w-full ${errors.ssl_cert_url && touched.ssl_cert_url ? 'border-red-500' : ''}`}
              placeholder="https://example.com/cert.pem"
            />
            {errors.ssl_cert_url && touched.ssl_cert_url && (
              <p className="text-red-500 text-xs mt-1">{errors.ssl_cert_url}</p>
            )}
          </div>
          
          <div className="mb-4">
            <label className="block font-medium mb-1 text-sm">SSL Key URL</label>
            <p className="text-gray-600 text-xs mb-1">URL to your private key file (.pem or .key)</p>
            <input
              type="text"
              name="ssl_key_url"
              value={formData.ssl_key_url || ''}
              onChange={handleChange}
              onBlur={handleFieldBlur}
              className={`neo-input w-full ${errors.ssl_key_url && touched.ssl_key_url ? 'border-red-500' : ''}`}
              placeholder="https://example.com/key.pem"
            />
            {errors.ssl_key_url && touched.ssl_key_url && (
              <p className="text-red-500 text-xs mt-1">{errors.ssl_key_url}</p>
            )}
          </div>
          
          <div className="mb-2">
            <label className="block font-medium mb-1 text-sm">SSL Root Certificate URL</label>
            <p className="text-gray-600 text-xs mb-1">URL to the CA certificate file (.pem or .crt)</p>
            <input
              type="text"
              name="ssl_root_cert_url"
              value={formData.ssl_root_cert_url || ''}
              onChange={handleChange}
              onBlur={handleFieldBlur}
              className={`neo-input w-full ${errors.ssl_root_cert_url && touched.ssl_root_cert_url ? 'border-red-500' : ''}`}
              placeholder="https://example.com/ca.pem"
            />
            {errors.ssl_root_cert_url && touched.ssl_root_cert_url && (
              <p className="text-red-500 text-xs mt-1">{errors.ssl_root_cert_url}</p>
            )}
          </div>
        </div>
      )}
    </>
  );
};

export default BasicConnectionTab; 