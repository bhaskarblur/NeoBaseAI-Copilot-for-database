import { AlertCircle, Boxes, KeyRound, Loader, UserRound } from 'lucide-react';
import React, { useState } from 'react';
import { LoginFormData } from '../../types/auth';
import analyticsService from '../../services/analyticsService';

interface LoginFormProps {
  onLogin: (data: LoginFormData) => Promise<void>;
  onSwitchToSignup: () => void;
  onSwitchToForgotPassword: () => void;
}

interface FormErrors {
  usernameOrEmail?: string;
  password?: string;
}

export default function LoginForm({ onLogin, onSwitchToSignup, onSwitchToForgotPassword }: LoginFormProps) {
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState<LoginFormData>({
    usernameOrEmail: '',
    password: ''
  });
  const [formError, setFormError] = useState<string | null>(null);

  const validateUsernameOrEmail = (usernameOrEmail: string) => {
    if (!usernameOrEmail) return 'Username or email is required';
    if (usernameOrEmail.length < 3) return 'Username or email must be at least 3 characters';
    return '';
  };

  const validatePassword = (password: string) => {
    if (!password) return 'Password is required';
    if (password.length < 6) {
      return 'Password must be at least 6 characters';
    }
    return '';
  };

  const validateForm = () => {
    const newErrors: FormErrors = {};

    const usernameOrEmailError = validateUsernameOrEmail(formData.usernameOrEmail);
    if (usernameOrEmailError) newErrors.usernameOrEmail = usernameOrEmailError;

    const passwordError = validatePassword(formData.password);
    if (passwordError) newErrors.password = passwordError;

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrors({});
    setFormError(null);

    if (!validateForm()) return;

    setIsLoading(true);
    try {
      await onLogin(formData);
      analyticsService.trackEvent('login_attempt', { usernameOrEmail: formData.usernameOrEmail });
    } catch (error: any) {
      setFormError(error.message);
      analyticsService.trackEvent('login_error', { error: error.message });
    } finally {
      setIsLoading(false);
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;

    setFormData(prev => ({
      ...prev,
      [name]: value
    }));

    if (touched[name]) {
      if (name === 'usernameOrEmail') {
        const trimmedValue = value.trim();
        const error = validateUsernameOrEmail(trimmedValue);
        setErrors(prev => ({ ...prev, usernameOrEmail: error }));
      } else if (name === 'password') {
        const error = validatePassword(value);
        setErrors(prev => ({ ...prev, password: error }));
      }
    }
  };

  const handleBlur = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setTouched(prev => ({ ...prev, [name]: true }));

    if (name === 'usernameOrEmail') {
      const trimmedValue = value.trim();
      if (trimmedValue !== value) {
        setFormData(prev => ({
          ...prev,
          [name]: trimmedValue
        }));
      }
      const error = validateUsernameOrEmail(trimmedValue);
      setErrors(prev => ({ ...prev, usernameOrEmail: error }));
    } else if (name === 'password') {
      const error = validatePassword(value);
      setErrors(prev => ({ ...prev, password: error }));
    }
  };

  return (
    <div className="min-h-screen bg-[#FFDB58]/20 flex items-center justify-center p-4">
      <div className="w-full max-w-md neo-border bg-white p-4 md:p-8">
        <h1 className="text-2xl md:text-3xl font-bold text-center mb-4 flex items-center justify-center gap-2">
          <Boxes className="w-10 h-10" />
          NeoBase
        </h1>
        <p className="text-gray-600 text-center mb-8">
          Welcome back to the NeoBase. Login to Continue
        </p>

        {formError && (
          <div className="mb-6 p-4 bg-red-50 border-2 border-red-500 rounded-lg">
            <div className="flex items-center gap-2 text-red-600">
              <AlertCircle className="w-5 h-5" />
              <p className="font-medium">{formError}</p>
            </div>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          <div>
            <div className="relative">
              <UserRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
              <input
                type="text"
                name="usernameOrEmail"
                placeholder="Username or Email"
                value={formData.usernameOrEmail}
                onChange={handleChange}
                onBlur={handleBlur}
                className={`neo-input pl-12 w-full ${errors.usernameOrEmail && touched.usernameOrEmail ? 'border-neo-error' : ''
                  }`}
                required
              />
            </div>
            {errors.usernameOrEmail && touched.usernameOrEmail && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.usernameOrEmail}</span>
              </div>
            )}
          </div>

          <div>
            <div className="relative">
              <KeyRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
              <input
                type="password"
                name="password"
                placeholder="Password"
                value={formData.password}
                onChange={handleChange}
                onBlur={handleBlur}
                className={`neo-input pl-12 w-full ${errors.password && touched.password ? 'border-neo-error' : ''
                  }`}
                required
              />
            </div>
            {errors.password && touched.password && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.password}</span>
              </div>
            )}
          </div>

          <button
            type="submit"
            className="neo-button w-full relative"
            disabled={isLoading}
          >
            {isLoading ? (
              <div className="flex items-center justify-center">
                <Loader className="w-4 h-4 animate-spin text-gray-400 mr-2" />
                Logging in...
              </div>
            ) : (
              'Login'
            )}
          </button>

          <div className="text-center mt-2">
            <button
              type="button"
              onClick={onSwitchToForgotPassword}
              className="text-green-600 hover:text-green-800 underline text-sm transition-colors duration-200 font-medium"
              disabled={isLoading}
            >
              Forgot your password?
            </button>
          </div>

          <button
            type="button"
            onClick={onSwitchToSignup}
            className="neo-button-secondary w-full"
            disabled={isLoading}
          >
            New User - Sign Up
          </button>
        </form>
      </div>
    </div>
  );
}