import { Chat } from '../types/chat';
import { Lock, Unlock, Shield } from 'lucide-react';

interface ConnectionDetailsProps {
  chat: Chat;
}

export default function ConnectionDetails({ chat }: ConnectionDetailsProps) {
  const { connection } = chat;

  return (
    <div className="bg-white rounded-lg shadow-sm p-4 mb-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-lg font-semibold">Connection Details</h3>
        {connection.use_ssl ? (
          <div className="flex items-center text-green-600">
            <Lock size={16} className="mr-1" />
            <span className="text-xs font-medium">SSL Enabled</span>
          </div>
        ) : (
          <div className="flex items-center text-amber-600">
            <Unlock size={16} className="mr-1" />
            <span className="text-xs font-medium">SSL Disabled</span>
          </div>
        )}
      </div>
      
      <div className="grid grid-cols-2 gap-3 text-sm">
        <div>
          <p className="text-gray-500 mb-1">Type</p>
          <p className="font-medium">{connection.type}</p>
        </div>
        <div>
          <p className="text-gray-500 mb-1">Host</p>
          <p className="font-medium">{connection.host}</p>
        </div>
        <div>
          <p className="text-gray-500 mb-1">Port</p>
          <p className="font-medium">{connection.port}</p>
        </div>
        <div>
          <p className="text-gray-500 mb-1">Database</p>
          <p className="font-medium">{connection.database}</p>
        </div>
        <div>
          <p className="text-gray-500 mb-1">Username</p>
          <p className="font-medium">{connection.username}</p>
        </div>
        <div>
          <p className="text-gray-500 mb-1">Password</p>
          <p className="font-medium">••••••••</p>
        </div>
      </div>
      
      {connection.use_ssl && (
        <div className="mt-4 pt-3 border-t border-gray-100">
          <div className="flex items-center mb-2">
            <Shield size={16} className="text-green-600 mr-2" />
            <h4 className="font-medium">SSL/TLS Configuration</h4>
          </div>
          
          <div className="grid grid-cols-1 gap-2 text-sm pl-6">
            {connection.ssl_cert_url && (
              <div>
                <p className="text-gray-500 mb-1">Certificate</p>
                <p className="font-medium text-xs truncate">{connection.ssl_cert_url}</p>
              </div>
            )}
            
            {connection.ssl_key_url && (
              <div>
                <p className="text-gray-500 mb-1">Key</p>
                <p className="font-medium text-xs truncate">{connection.ssl_key_url}</p>
              </div>
            )}
            
            {connection.ssl_root_cert_url && (
              <div>
                <p className="text-gray-500 mb-1">CA Certificate</p>
                <p className="font-medium text-xs truncate">{connection.ssl_root_cert_url}</p>
              </div>
            )}
            
            {!connection.ssl_cert_url && !connection.ssl_key_url && !connection.ssl_root_cert_url && (
              <p className="text-xs text-gray-500">Using server's default SSL configuration</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
} 