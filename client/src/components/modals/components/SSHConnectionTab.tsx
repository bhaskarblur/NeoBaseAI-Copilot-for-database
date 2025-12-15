import React, { useState, useEffect } from 'react';
import { AlertCircle } from 'lucide-react';
import { Connection, SSHAuthMethod } from '../../../types/chat';

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
  // State for SSH authentication method - sync with formData
  const [sshAuthMethod, setSshAuthMethod] = useState<SSHAuthMethod>(
    (formData.ssh_auth_method as SSHAuthMethod) || SSHAuthMethod.PublicKey
  );
  // State to track the connection URI value independently
  const [connectionUri, setConnectionUri] = useState<string>('');

  // Sync local state with formData when it changes
  useEffect(() => {
    if (formData.ssh_auth_method) {
      setSshAuthMethod(formData.ssh_auth_method as SSHAuthMethod);
    }
  }, [formData.ssh_auth_method]);

  // Handle auth method change and update formData
  const handleAuthMethodChange = (method: SSHAuthMethod) => {
    setSshAuthMethod(method);
    // Create synthetic event to update formData
    const syntheticEvent = {
      target: {
        name: 'ssh_auth_method',
        value: method
      }
    } as React.ChangeEvent<HTMLInputElement>;
    handleChange(syntheticEvent);
  };

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

  // Get URI config based on database type
  const getUriConfig = (dbType: string): { label: string; placeholder: string; description: string } | null => {
    switch (dbType) {
      case 'postgresql':
      case 'yugabytedb':
        return {
          label: 'PostgreSQL Connection URI',
          placeholder: 'postgresql://username:password@host:port/database',
          description: 'Paste your PostgreSQL connection string to auto-fill fields'
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
        break;
      case 'mysql':
        uri = `mysql://${username}${username ? ':' + password : ''}@${host}${port ? ':' + port : ''}/${database}`;
        break;
      case 'clickhouse':
        uri = `clickhouse://${username}${username ? ':' + password : ''}@${host}${port ? ':' + port : ''}/${database}`;
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

  // Parse connection URI for different database types
  const parsePostgreSQLUri = (uri: string): Partial<Connection> => {
    try {
      const match = uri.match(/postgresql:\/\/(?:([^:@]+)(?::([^@]*))?@)?([^:/?]+)(?::(\d+))?\/([^?]+)(?:\?(.*))?/);
      if (match) {
        return {
          username: match[1] || '',
          password: match[2] || '',
          host: match[3],
          port: match[4] || '5432',
          database: match[5]
        };
      }
    } catch (e) {
      console.error('Error parsing PostgreSQL URI:', e);
    }
    return {};
  };

  const parseMySQLUri = (uri: string): Partial<Connection> => {
    try {
      const match = uri.match(/mysql:\/\/(?:([^:@]+)(?::([^@]*))?@)?([^:/?]+)(?::(\d+))?\/([^?]+)/);
      if (match) {
        return {
          username: match[1] || '',
          password: match[2] || '',
          host: match[3],
          port: match[4] || '3306',
          database: match[5]
        };
      }
    } catch (e) {
      console.error('Error parsing MySQL URI:', e);
    }
    return {};
  };

  const parseClickHouseUri = (uri: string): Partial<Connection> => {
    try {
      const match = uri.match(/clickhouse:\/\/(?:([^:@]+)(?::([^@]*))?@)?([^:/?]+)(?::(\d+))?\/([^?]+)/);
      if (match) {
        return {
          username: match[1] || '',
          password: match[2] || '',
          host: match[3],
          port: match[4] || '8123',
          database: match[5]
        };
      }
    } catch (e) {
      console.error('Error parsing ClickHouse URI:', e);
    }
    return {};
  };

  const parseMongoDBUri = (uri: string): Partial<Connection> => {
    try {
      const srvFormat = uri.startsWith('mongodb+srv://');
      const protocolMatch = uri.match(/^(mongodb(?:\+srv)?:\/\/)(.*)/);
      if (!protocolMatch) return {};

      const [, , remainder] = protocolMatch;
      const hasCredentials = remainder.includes('@');
      let username = '';
      let password = '';
      let hostPart = remainder;

      if (hasCredentials) {
        const lastAtIndex = remainder.lastIndexOf('@');
        const credentialsPart = remainder.substring(0, lastAtIndex);
        hostPart = remainder.substring(lastAtIndex + 1);

        const firstColonIndex = credentialsPart.indexOf(':');
        if (firstColonIndex !== -1) {
          username = decodeURIComponent(credentialsPart.substring(0, firstColonIndex));
          password = decodeURIComponent(credentialsPart.substring(firstColonIndex + 1));
        } else {
          username = decodeURIComponent(credentialsPart);
        }
      }

      let host = '';
      let port = srvFormat ? '27017' : '';
      let database = 'test';
      let authDatabase = 'admin';

      const pathIndex = hostPart.indexOf('/');
      if (pathIndex !== -1) {
        const hostPortPart = hostPart.substring(0, pathIndex);
        const pathPart = hostPart.substring(pathIndex + 1);

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

        const portIndex = hostPortPart.indexOf(':');
        if (portIndex !== -1) {
          host = hostPortPart.substring(0, portIndex);
          port = hostPortPart.substring(portIndex + 1);
        } else {
          host = hostPortPart;
        }
      } else {
        const portIndex = hostPart.indexOf(':');
        if (portIndex !== -1) {
          host = hostPart.substring(0, portIndex);
          port = hostPart.substring(portIndex + 1);
        } else {
          host = hostPart;
        }
      }

      return {
        username: username || '',
        password: password || '',
        host,
        port: port || (srvFormat ? '27017' : ''),
        database: database || 'test',
        auth_database: authDatabase
      };
    } catch (e) {
      console.error('Error parsing MongoDB URI:', e);
    }
    return {};
  };

  const parseConnectionUri = (uri: string, dbType: string) => {
    try {
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
      console.error(`Invalid ${dbType} URI format`, err);
    }
  };

  // Update connection URI when form data changes
  useEffect(() => {
    if (formData.host && formData.database) {
      setConnectionUri(buildConnectionUri(formData));
    } else {
      // Clear connection URI when type changes and no host/database are set
      setConnectionUri('');
    }
  }, [formData.host, formData.database, formData.username, formData.port, formData.type, formData.auth_database]);

  return (
    <>
      {/* SSH Tunnel Configuration */}
      <div className="mb-6 p-4 border-dashed border-2 border-gray-300 rounded-lg bg-gray-50">
            <h3 className="font-bold mb-3 text-md">SSH Tunnel Configuration</h3>
            
            <div className="mb-4">
              <label className="block font-medium mb-2 text-sm">SSH Host</label>
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
              <label className="block font-medium mb-2 text-sm">SSH Port</label>
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
            
            <div className="mb-6">
              <label className="block font-medium mb-2 text-sm">SSH Username</label>
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

            {/* SSH Authentication Method Selector */}
            <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-lg">
              <label className="block font-medium mb-2 text-sm">Authentication Method</label>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    value={SSHAuthMethod.PublicKey}
                    checked={sshAuthMethod === SSHAuthMethod.PublicKey}
                    onChange={(e) => handleAuthMethodChange(e.target.value as SSHAuthMethod)}
                    className="w-4 h-4"
                  />
                  <span className="text-sm">Public Key</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    value={SSHAuthMethod.Password}
                    checked={sshAuthMethod === SSHAuthMethod.Password}
                    onChange={(e) => handleAuthMethodChange(e.target.value as SSHAuthMethod)}
                    className="w-4 h-4"
                  />
                  <span className="text-sm">Password</span>
                </label>
              </div>
            </div>

            {/* Public Key Authentication Fields */}
            {sshAuthMethod === SSHAuthMethod.PublicKey && (
              <>
                <div className="mb-4">
                  <label className="block font-medium mb-2 text-sm">SSH Private Key</label>
                  <p className="text-gray-600 text-xs mb-1">Paste your OpenSSH private key (format: -----BEGIN OPENSSH PRIVATE KEY-----)</p>
                  {/* <p className="text-gray-500 text-xs mb-2 bg-gray-50 p-2 rounded border border-gray-200">
                    💡 We support OpenSSH format keys. If you have a legacy RSA format key (-----BEGIN RSA PRIVATE KEY-----), please convert it first.
                  </p> */}
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
                    onBlur={() => {
                      const mockEvent = {
                        target: {
                          name: 'ssh_private_key'
                        }
                      } as React.FocusEvent<HTMLInputElement>;
                      handleBlur(mockEvent);
                      validateField('ssh_private_key', formData);
                    }}
                    className={`neo-input w-full font-mono text-xs ${errors.ssh_private_key && touched.ssh_private_key ? 'border-red-500' : ''}`}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----
...
...
-----END OPENSSH PRIVATE KEY-----"
                    rows={6}
                  />
                  {errors.ssh_private_key && touched.ssh_private_key && (
                    <p className="text-red-500 text-xs mt-1">{errors.ssh_private_key}</p>
                  )}
                </div>
                
                <div className="mb-4 p-3 bg-gray-50 border border-gray-200 rounded-lg">
                  <label className="block font-medium mb-2 text-sm">Alternatively, Load Private Key from URL</label>
                  <p className="text-gray-600 text-xs mb-1">You can provide a secure URL to download your private key</p>
                  <input
                    type="text"
                    name="ssh_private_key_url"
                    value={formData.ssh_private_key_url || ''}
                    onChange={handleChange}
                    className="neo-input w-full text-sm"
                    placeholder="e.g. https://example.com/keys/private-key.pem"
                  />
                  {/* <p className="text-gray-500 text-xs mt-3">⚠️ Only use this if you fully trust the URL source</p> */}
                </div>
                
                <div className="mb-4">
                  <label className="block font-medium mb-2 text-sm">SSH Passphrase (Optional)</label>
                  <p className="text-gray-600 text-xs mb-1">If your private key is encrypted with a passphrase</p>
                  <input
                    type="password"
                    name="ssh_passphrase"
                    value={formData.ssh_passphrase || ''}
                    onChange={handleChange}
                    className="neo-input w-full text-base"
                    placeholder="Leave empty if your key doesn't have a passphrase"
                  />
                </div>
              </>
            )}

            {/* Password Authentication Fields */}
            {sshAuthMethod === SSHAuthMethod.Password && (
              <div className="mb-4">
                <label className="block font-medium mb-2 text-sm">SSH Password</label>
                <p className="text-gray-600 text-xs mb-1">Your SSH password for authentication</p>
                <input
                  type="password"
                  name="ssh_password"
                  value={formData.ssh_password || ''}
                  onChange={handleChange}
                  className="neo-input w-full"
                  placeholder="Enter your SSH password"
                />
                {/* <p className="text-gray-500 text-xs mt-2 bg-yellow-50 p-2 rounded border border-yellow-200">
                  ⚠️ Password-based authentication is less secure than public key authentication. Consider using keys when possible.
                </p> */}
              </div>
            )}
          </div>

          {/* Database Settings Section */}
          <div>
            <h3 className="font-bold text-lg mb-3">Data Source Credentials</h3>
            
            {/* Universal Connection URI Field - Show for supported database types */}
            {getUriConfig(formData.type) && formData.type !== 'spreadsheet' && (
              <div className="mb-6">
                <label className="block font-bold mb-2 text-lg">{getUriConfig(formData.type)!.label}</label>
                <p className="text-gray-600 text-sm mb-2">{getUriConfig(formData.type)!.description}</p>
                <input
                  type="text"
                  name="connection_uri"
                  ref={formData.type === 'mongodb' ? mongoUriSshInputRef : undefined}
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
            
            {/* Host */}
            <div className="mb-6">
              <label className="block font-bold mb-2 text-lg">Host</label>
              <p className="text-gray-600 text-sm mb-2">The hostname or IP address of your database server</p>
              <input
                type="text"
                name="host"
                value={formData.host}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.host && touched.host ? 'border-red-500' : ''}`}
                placeholder="e.g., localhost, db.example.com, 192.168.1.1"
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
              <label className="block font-bold mb-2 text-lg">Port</label>
              <p className="text-gray-600 text-sm mb-2">The port number your database is listening on</p>
              <input
                type="text"
                name="port"
                value={formData.port}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.port && touched.port ? 'border-red-500' : ''}`}
                placeholder="e.g., 5432 (PostgreSQL), 3306 (MySQL), 27017 (MongoDB)"
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
              <label className="block font-bold mb-2 text-lg">Database Name</label>
              <p className="text-gray-600 text-sm mb-2">The name of the specific database to connect to</p>
              <input
                type="text"
                name="database"
                value={formData.database}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.database && touched.database ? 'border-red-500' : ''}`}
                placeholder="e.g., myapp_production, users_db"
              />
              {errors.database && touched.database && (
                <div className="flex items-center gap-1 mt-1 text-red-500 text-sm">
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
            
            {/* Username */}
            <div className="mb-6">
              <label className="block font-bold mb-2 text-lg">Username</label>
              <p className="text-gray-600 text-sm mb-2">Database user with appropriate permissions</p>
              <input
                type="text"
                name="username"
                value={formData.username}
                onChange={handleChange}
                onBlur={handleFieldBlur}
                className={`neo-input w-full ${errors.username && touched.username ? 'border-red-500' : ''}`}
                placeholder="e.g., db_user, assistant"
              />
              <p className="text-gray-500 text-sm mt-3">Please be careful about the user credentials you provide with only those permissions that you need, such as read-only, read-write or limited access.</p>
              {errors.username && touched.username && (
                <div className="flex items-center gap-1 mt-1 text-red-500 text-sm">
                  <AlertCircle className="w-4 h-4" />
                  <span>{errors.username}</span>
                </div>
              )}
            </div>
            
            {/* Password */}
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
              <p className="text-gray-500 text-sm mt-3">Leave blank if the database has no password, but it's recommended to set a password for the database user</p>
            </div>

          </div>

    </>
  );
};

export default SSHConnectionTab; 