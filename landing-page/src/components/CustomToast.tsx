import { toast as hotToast, ToastOptions } from 'react-hot-toast';

// Toast styles matching the client design exactly
const baseToastStyle = {
  background: '#000',
  color: '#fff',
  border: '4px solid #000',
  borderRadius: '12px',
  boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
  padding: '12px 24px',
  fontSize: '14px',
  fontWeight: '500',
  fontFamily: 'Archivo, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
} as const;

const successToastStyle: ToastOptions = {
  style: baseToastStyle,
  duration: 2000,
  icon: '✅',
};

const errorToastStyle: ToastOptions = {
  style: {
    ...baseToastStyle,
    background: '#ff4444',
    border: '4px solid #cc0000',
    color: '#fff',
    fontWeight: '500',
  },
  duration: 2000,
  icon: '❌',
};

// Custom toast functions
export const toast = {
  success: (message: string) => {
    hotToast.success(message, successToastStyle);
  },
  error: (message: string) => {
    hotToast.error(message, errorToastStyle);
  },
};

// Export the styles for direct use if needed
export { successToastStyle, errorToastStyle };