import { Connection, SSHAuthMethod } from '../types/chat';

export interface FormErrors {
  host?: string;
  port?: string;
  database?: string;
  username?: string;
  ssl_cert_url?: string;
  ssl_key_url?: string;
  ssl_root_cert_url?: string;
  ssh_host?: string;
  ssh_port?: string;
  ssh_username?: string;
  ssh_private_key?: string;
}

export const isValidUrl = (url: string): boolean => {
  try {
    new URL(url);
    return true;
  } catch (e) {
    return false;
  }
};

export const validateField = (name: string, value: Connection): string => {
  switch (name) {
    case 'host':
      if (!value.host.trim()) {
        return 'Host is required';
      }
      if (!/^[a-zA-Z0-9.-]+$/.test(value.host)) {
        return 'Invalid host format';
      }
      break;
    case 'port':
      // For MongoDB, port is optional and can be empty
      if (value.type === 'mongodb') {
        return '';
      }
      if (!value.port) {
        return 'Port is required';
      }
      // eslint-disable-next-line no-case-declarations
      const port = parseInt(value.port);
      if (isNaN(port) || port < 1 || port > 65535) {
        return 'Port must be between 1 and 65535';
      }
      break;
    case 'database':
      if (!value.database.trim()) {
        return 'Database name is required';
      }
      if (!/^[a-zA-Z0-9_-]+$/.test(value.database)) {
        return 'Invalid database name format';
      }
      break;
    case 'username':
      if (!value.username.trim()) {
        return 'Username is required';
      }
      break;
    case 'ssl_cert_url':
      if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_cert_url?.trim()) {
        return 'SSL Certificate URL is required for this SSL mode';
      }
      if (value.ssl_cert_url && !isValidUrl(value.ssl_cert_url)) {
        return 'Invalid URL format';
      }
      break;
    case 'ssl_key_url':
      if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_key_url?.trim()) {
        return 'SSL Key URL is required for this SSL mode';
      }
      if (value.ssl_key_url && !isValidUrl(value.ssl_key_url)) {
        return 'Invalid URL format';
      }
      break;
    case 'ssl_root_cert_url':
      if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_root_cert_url?.trim()) {
        return 'SSL Root Certificate URL is required for this SSL mode';
      }
      if (value.ssl_root_cert_url && !isValidUrl(value.ssl_root_cert_url)) {
        return 'Invalid URL format';
      }
      break;
    case 'ssh_host':
      if (value.ssh_enabled && !value.ssh_host?.trim()) {
        return 'SSH Host is required';
      }
      if (value.ssh_host && !/^[a-zA-Z0-9.-]+$/.test(value.ssh_host)) {
        return 'Invalid SSH host format';
      }
      break;
    case 'ssh_port':
      if (value.ssh_enabled && !value.ssh_port) {
        return 'SSH Port is required';
      }
      if (value.ssh_port) {
        // eslint-disable-next-line no-case-declarations
        const sshPort = parseInt(value.ssh_port);
        if (isNaN(sshPort) || sshPort < 1 || sshPort > 65535) {
          return 'SSH Port must be between 1 and 65535';
        }
      }
      break;
    case 'ssh_username':
      if (value.ssh_enabled && !value.ssh_username?.trim()) {
        return 'SSH Username is required';
      }
      break;
    case 'ssh_private_key':
      if (value.ssh_enabled && value.ssh_auth_method === SSHAuthMethod.PublicKey && !value.ssh_private_key?.trim()) {
        return 'SSH Private Key is required for public key authentication';
      }
      break;
    default:
      return '';
  }
  return '';
};

export const formatConnectionString = (connection: Connection): string => {
  let result = `DATABASE_TYPE=${connection.type}
DATABASE_HOST=${connection.host}
DATABASE_PORT=${connection.port}
DATABASE_NAME=${connection.database}
DATABASE_USERNAME=${connection.username}
DATABASE_PASSWORD=`; // Mask password

  if (connection.use_ssl) {
    result += `\nUSE_SSL=true`;
    result += `\nSSL_MODE=${connection.ssl_mode || 'disable'}`;

    if (connection.ssl_cert_url) {
      result += `\nSSL_CERT_URL=${connection.ssl_cert_url}`;
    }

    if (connection.ssl_key_url) {
      result += `\nSSL_KEY_URL=${connection.ssl_key_url}`;
    }

    if (connection.ssl_root_cert_url) {
      result += `\nSSL_ROOT_CERT_URL=${connection.ssl_root_cert_url}`;
    }
  }

  if (connection.ssh_enabled) {
    result += `\nSSH_ENABLED=true`;
    result += `\nSSH_HOST=${connection.ssh_host || ''}`;
    result += `\nSSH_PORT=${connection.ssh_port || '22'}`;
    result += `\nSSH_USERNAME=${connection.ssh_username || ''}`;
    result += `\nSSH_PRIVATE_KEY=`; // Mask private key

    if (connection.ssh_passphrase) {
      result += `\nSSH_PASSPHRASE=`; // Mask passphrase
    }
  }

  return result;
};
