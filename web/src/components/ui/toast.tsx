import { Toaster, toast as hotToast } from 'react-hot-toast';
import { FailedIcon, NoticeIcon, OkIcon, WarningIcon } from '@/components/icons'

export const ToastProvider = () => {
  return (
    <Toaster
      position="top-right"
      toastOptions={{
        duration: 4000,
        style: {
          background: '#1f2937',
          color: '#fff',
          border: '1px solid #374151',
        },
        success: {
          iconTheme: {
            primary: '#10b981',
            secondary: '#fff',
          },
        },
        error: {
          iconTheme: {
            primary: '#ef4444',
            secondary: '#fff',
          },
        },
      }}
    />
  );
};

// Component for rendering toast messages (used by AppShell)
export const Toasts = () => null;

// Custom toast functions with icons
export const toast = {
  success: (message: string) => {
    hotToast.custom(() => (
      <div className="flex items-center gap-3 bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 shadow-lg">
        <OkIcon className="w-5 h-5 text-green-500 flex-shrink-0" />
        <span className="text-sm text-white">{message}</span>
      </div>
    ));
  },
  
  error: (message: string) => {
    hotToast.custom(() => (
      <div className="flex items-center gap-3 bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 shadow-lg">
        <FailedIcon className="w-5 h-5 text-red-500 flex-shrink-0" />
        <span className="text-sm text-white">{message}</span>
      </div>
    ));
  },
  
  info: (message: string) => {
    hotToast.custom(() => (
      <div className="flex items-center gap-3 bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 shadow-lg">
        <NoticeIcon className="w-5 h-5 text-blue-500 flex-shrink-0" />
        <span className="text-sm text-white">{message}</span>
      </div>
    ));
  },
  
  warning: (message: string) => {
    hotToast.custom(() => (
      <div className="flex items-center gap-3 bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 shadow-lg">
        <WarningIcon className="w-5 h-5 text-yellow-500 flex-shrink-0" />
        <span className="text-sm text-white">{message}</span>
      </div>
    ));
  },
};

// Compatibility wrapper for existing code - a real function
export function pushToast(message: string, type: 'success' | 'error' | 'info' | 'warning' = 'success'): void {
  toast[type](message);
}