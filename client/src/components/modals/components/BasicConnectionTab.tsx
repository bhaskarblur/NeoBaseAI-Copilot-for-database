import React, { useState, useEffect } from 'react';
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
  // State to track the connection URI value independently
  const [connectionUri, setConnectionUri] = useState<string>('');

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

  // Unified function to parse connection URIs for different database types
  const parseConnectionUri = (uri: string, dbType: string) => {
    try {
      // Remove whitespace
      uri = uri.trim();
      if (!uri) return;

      let parsedData: Partial<Connection> = {};

      switch (dbType) {
        case 'postgresql':
        case 'yugabytedb':
          parsedData = parsePostgreSQLUri(uri);
          break;
        case 'mysql':
          parsedData = parseMySQLUri(uri);
          break;
        case 'clickhouse':
          parsedData = parseClickHouseUri(uri);
          break;
        case 'mongodb':
          parsedData = parseMongoDBUri(uri);
          break;
        default:
          console.log(`URI parsing not supported for database type: ${dbType}`);
          return;
      }

      if (parsedData && Object.keys(parsedData).length > 0) {
        console.log(`${dbType} URI parsed successfully`, parsedData);
        
        // Update formData through parent component
        Object.entries(parsedData).forEach(([key, value]) => {
          if (value !== undefined && value !== null) {
            const mockEvent = {
              target: { name: key, value: String(value) }
            } as React.ChangeEvent<HTMLInputElement>;
            handleChange(mockEvent);
          }
        });
      }
    } catch (err) {
      console.log(`Invalid ${dbType} URI format`, err);
    }
  };

  // PostgreSQL URI parser: postgresql://user:password@host:port/database?params
  const parsePostgreSQLUri = (uri: string): Partial<Connection> => {
    const match = uri.match(/^postgresql:\/\/(?:([^:]+)(?::([^@]+))?@)?([^:\/]+)(?::(\d+))?(?:\/([^?]+))?(?:\?(.*))?$/);
    if (!match) throw new Error('Invalid PostgreSQL URI format');

    const [, username, password, host, port, database, params] = match;
    const result: Partial<Connection> = {};

    if (host) result.host = host;
    if (port) result.port = port;
    if (database) result.database = database;
    if (username) result.username = decodeURIComponent(username);
    if (password) result.password = decodeURIComponent(password);

    // Parse SSL parameters
    if (params) {
      const urlParams = new URLSearchParams(params);
      if (urlParams.has('sslmode')) {
        result.use_ssl = urlParams.get('sslmode') !== 'disable';
        result.ssl_mode = urlParams.get('sslmode') as any;
      }
    }

    return result;
  };

  // MySQL URI parser: mysql://user:password@host:port/database?params
  const parseMySQLUri = (uri: string): Partial<Connection> => {
    const match = uri.match(/^mysql:\/\/(?:([^:]+)(?::([^@]+))?@)?([^:\/]+)(?::(\d+))?(?:\/([^?]+))?(?:\?(.*))?$/);
    if (!match) throw new Error('Invalid MySQL URI format');

    const [, username, password, host, port, database, params] = match;
    const result: Partial<Connection> = {};

    if (host) result.host = host;
    if (port) result.port = port;
    if (database) result.database = database;
    if (username) result.username = decodeURIComponent(username);
    if (password) result.password = decodeURIComponent(password);

    // Parse SSL parameters
    if (params) {
      const urlParams = new URLSearchParams(params);
      if (urlParams.has('useSSL')) {
        result.use_ssl = urlParams.get('useSSL') === 'true';
      }
    }

    return result;
  };

  // ClickHouse URI parser: clickhouse://user:password@host:port/database?params
  const parseClickHouseUri = (uri: string): Partial<Connection> => {
    const match = uri.match(/^clickhouse:\/\/(?:([^:]+)(?::([^@]+))?@)?([^:\/]+)(?::(\d+))?(?:\/([^?]+))?(?:\?(.*))?$/);
    if (!match) throw new Error('Invalid ClickHouse URI format');

    const [, username, password, host, port, database, params] = match;
    const result: Partial<Connection> = {};

    if (host) result.host = host;
    if (port) result.port = port;
    if (database) result.database = database;
    if (username) result.username = decodeURIComponent(username);
    if (password) result.password = decodeURIComponent(password);

    // Parse SSL parameters
    if (params) {
      const urlParams = new URLSearchParams(params);
      if (urlParams.has('secure')) {
        result.use_ssl = urlParams.get('secure') === 'true' || urlParams.get('secure') === '1';
      }
    }

    return result;
  };

  // MongoDB URI parser (existing logic refactored)
  const parseMongoDBUri = (uri: string): Partial<Connection> => {
    const srvFormat = uri.startsWith('mongodb+srv://');
    
    // Extract the protocol and the rest
    const protocolMatch = uri.match(/^(mongodb(?:\+srv)?:\/\/)(.*)/);
    if (!protocolMatch) {
      throw new Error('Invalid MongoDB URI format: Missing protocol');
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
    
    if (!host) throw new Error('Could not extract host from MongoDB URI');
    
    return {
      host: host,
      port: port || (srvFormat ? '27017' : formData.port),
      database: database || 'test',
      auth_database: authDatabase,
      username: username || formData.username,
      password: password || formData.password
    };
  };

  // Get connection URI placeholder and label based on database type
  const getUriConfig = (dbType: string) => {
    switch (dbType) {
      case 'postgresql':
        return {
          label: 'PostgreSQL Connection URI',
          placeholder: 'postgresql://username:password@host:port/database',
          description: 'Paste your PostgreSQL connection string to auto-fill fields'
        };
      case 'yugabytedb':
        return {
          label: 'YugabyteDB Connection URI',
          placeholder: 'postgresql://username:password@host:port/database',
          description: 'Paste your YugabyteDB connection string to auto-fill fields'
        };
      case 'mysql':
        return {
          label: 'MySQL Connection URI',
          placeholder: 'mysql://username:password@host:port/database',
          description: 'Paste your MySQL connection string to auto-fill fields'
        };
      case 'clickhouse':
        return {
          label: 'ClickHouse Connection URI',
          placeholder: 'clickhouse://username:password@host:port/database',
          description: 'Paste your ClickHouse connection string to auto-fill fields'
        };
      case 'mongodb':
        return {
          label: 'MongoDB Connection URI',
          placeholder: 'mongodb://username:password@host:port/database or mongodb+srv://username:password@host/database',
          description: 'Paste your MongoDB connection string to auto-fill fields'
        };
      default:
        return null;
    }
  };

  // Build connection URI from form data
  const buildConnectionUri = (data: Connection): string => {
    if (!data.host || !data.database) return '';

    let uri = '';
    const username = data.username || '';
    // Always show <password> placeholder to indicate where password should go
    const password = '<password>';
    const host = data.host;
    const port = data.port || getDefaultPort(data.type);
    const database = data.database;

    switch (data.type) {
      case 'postgresql':
      case 'yugabytedb':
        uri = `postgresql://${username}${username ? ':' + password : ''}@${host}${port ? ':' + port : ''}/${database}`;
        if (data.use_ssl && data.ssl_mode && data.ssl_mode !== 'disable') {
          uri += `?sslmode=${data.ssl_mode}`;
        }
        break;
      case 'mysql':
        uri = `mysql://${username}${username ? ':' + password : ''}@${host}${port ? ':' + port : ''}/${database}`;
        if (data.use_ssl) {
          uri += '?useSSL=true';
        }
        break;
      case 'clickhouse':
        uri = `clickhouse://${username}${username ? ':' + password : ''}@${host}${port ? ':' + port : ''}/${database}`;
        if (data.use_ssl) {
          uri += '?secure=true';
        }
        break;
      case 'mongodb':
        uri = `mongodb://${username}${username ? ':' + password : ''}@${host}${port ? ':' + port : ''}/${database}`;
        if (data.auth_database && data.auth_database !== 'admin') {
          uri += `?authSource=${data.auth_database}`;
        }
        break;
      default:
        return '';
    }

    return uri;
  };

  // Get default port for database type
  const getDefaultPort = (dbType: string): string => {
    switch (dbType) {
      case 'postgresql':
      case 'yugabytedb':
        return '5432';
      case 'mysql':
        return '3306';
      case 'clickhouse':
        return '8123';
      case 'mongodb':
        return '27017';
      default:
        return '';
    }
  };

  // Update connection URI when form data changes
  useEffect(() => {
    if (formData.host && formData.database) {
      setConnectionUri(buildConnectionUri(formData));
    }
  }, [formData.host, formData.database, formData.username, formData.port, formData.type, formData.use_ssl, formData.ssl_mode, formData.auth_database]);

  return (
    <>

      {/* Show file upload instructions for CSV/Excel */}
      {(formData.type === 'spreadsheet') && (
        <div className="p-4 bg-blue-50 border-2 border-blue-200 rounded-lg">
          <p className="text-blue-800">
            Please continue to upload your CSV/Excel files in the next step.
            All connection details will be handled automatically.
          </p>
        </div>
      )}

      {/* Universal Connection URI Field - Show for supported database types */}
      {getUriConfig(formData.type) && formData.type !== 'spreadsheet' && (
        <div className="mb-6">
          <label className="block font-bold mb-2 text-lg">{getUriConfig(formData.type)!.label}</label>
          <p className="text-gray-600 text-sm mb-2">{getUriConfig(formData.type)!.description}</p>
          <input
            type="text"
            name="connection_uri"
            ref={formData.type === 'mongodb' ? mongoUriInputRef : undefined}
            className="neo-input w-full"
            placeholder={getUriConfig(formData.type)!.placeholder}
            value={connectionUri}
            onChange={(e) => {
              const uri = e.target.value;
              // Update local state to allow free editing
              setConnectionUri(uri);
              // Save the URI value through the callback for MongoDB compatibility
              if (formData.type === 'mongodb' && onMongoUriChange) {
                onMongoUriChange(uri);
              }
              // Parse the URI based on the selected database type
              parseConnectionUri(uri, formData.type);
            }}
          />
          <p className="text-gray-500 text-xs mt-3">
            Connection URI will be used to auto-fill the fields below. Replace &lt;password&gt; with your actual password.
            {formData.type === 'mongodb' && ' Both standard and Atlas SRV formats supported.'}
            {formData.type === 'postgresql' && ' Supports sslmode parameter (e.g., ?sslmode=require).'}
            {formData.type === 'yugabytedb' && ' Supports sslmode parameter (e.g., ?sslmode=require).'}
            {formData.type === 'mysql' && ' Supports useSSL parameter (e.g., ?useSSL=true).'}
            {formData.type === 'clickhouse' && ' Supports secure parameter (e.g., ?secure=true).'}
          </p>
        </div>
      )}

      {/* Divider between Connection URI and Manual Fields */}
      {getUriConfig(formData.type) && formData.type !== 'spreadsheet' && (
        <div className="flex items-center my-6">
          <div className="flex-1 border-t border-gray-300"></div>
          <span className="px-4 text-sm font-medium text-gray-500">OR</span>
          <div className="flex-1 border-t border-gray-300"></div>
        </div>
      )}

      {formData.type !== 'spreadsheet' && (
        <>
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
        <p className="text-gray-500 text-xs mt-3">Leave blank if the database has no password, but it's recommended to set a password for the database user</p>
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
      )}
    </>
  );
};

export default BasicConnectionTab; 