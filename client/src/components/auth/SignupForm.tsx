import { AlertCircle, Boxes, KeyRound, Loader, Lock, UserRound, Mail, Check } from 'lucide-react';
import React, { useState } from 'react';
import { SignupFormData } from '../../types/auth';
import analyticsService from '../../services/analyticsService';
import { initiateGoogleOAuth } from '../../utils/googleOAuth';
import authService from '../../services/authService';

interface SignupFormProps {
    onSignup: (data: SignupFormData) => Promise<void>;
    onSwitchToLogin: () => void;
}

interface FormErrors {
    username?: string;
    email?: string;
    password?: string;
    confirmPassword?: string;
    userSignupSecret?: string;
}

export default function SignupForm({ onSignup, onSwitchToLogin }: SignupFormProps) {
    const [errors, setErrors] = useState<FormErrors>({});
    const [touched, setTouched] = useState<Record<string, boolean>>({});
    const [isLoading, setIsLoading] = useState(false);
    const [formData, setFormData] = useState<SignupFormData>({
        username: '',
        email: '',
        password: '',
        confirmPassword: '',
        userSignupSecret: ''
    });
    const [formError, setFormError] = useState<string | null>(null);
    const Environment = import.meta.env.VITE_ENVIRONMENT;
    
    // Google OAuth states
    const [showGoogleSignupSecret, setShowGoogleSignupSecret] = useState(false);
    const [googleSignupSecret, setGoogleSignupSecret] = useState('');
    const [isValidatingSecret, setIsValidatingSecret] = useState(false);
    const [secretValidationStatus, setSecretValidationStatus] = useState<'valid' | 'invalid' | null>(null);

    const validateUsername = (username: string) => {
        if (!username) return 'Username is required';
        if (username.length < 3) return 'Username must be at least 3 characters';
        if (username.includes(' ')) return 'Username cannot contain spaces';
        return '';
    };

    const validateEmail = (email: string) => {
        if (!email) return 'Email is required';
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        if (!emailRegex.test(email)) return 'Please enter a valid email address';
        return '';
    };

    const validatePassword = (password: string) => {
        if (!password) return 'Password is required';
        if (password.length < 6) {
            return 'Password must be at least 6 characters';
        }
        return '';
    };

    const validateUserSignupSecret = (userSignupSecret: string) => {
        if (!userSignupSecret) return 'User signup secret is required';
        return '';
    };

    const validateForm = () => {
        const newErrors: FormErrors = {};

        const usernameError = validateUsername(formData.username);
        if (usernameError) newErrors.username = usernameError;

        const emailError = validateEmail(formData.email);
        if (emailError) newErrors.email = emailError;

        const passwordError = validatePassword(formData.password);
        if (passwordError) newErrors.password = passwordError;

        const userSignupSecretError = validateUserSignupSecret(formData.userSignupSecret);
        if (Environment !== "DEVELOPMENT" && userSignupSecretError) {
            newErrors.userSignupSecret = userSignupSecretError;
        }

        if (formData.password !== formData.confirmPassword) {
            newErrors.confirmPassword = 'Passwords do not match';
        }

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
            await onSignup(formData);
            analyticsService.trackEvent('signup_attempt', { username: formData.username, email: formData.email });
        } catch (error: any) {
            setFormError(error.message);
            analyticsService.trackEvent('signup_error', { error: error.message });
        } finally {
            setIsLoading(false);
        }
    };

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { name, value } = e.target;

        // Prevent spaces in username
        if (name === 'username' && value.includes(' ')) {
            const valueWithoutSpaces = value.replace(/\s/g, '');
            setFormData(prev => ({
                ...prev,
                [name]: valueWithoutSpaces
            }));

            if (touched[name]) {
                const error = validateUsername(valueWithoutSpaces);
                setErrors(prev => ({ ...prev, username: error }));
            }
            return;
        }

        setFormData(prev => ({
            ...prev,
            [name]: value
        }));

        if (touched[name]) {
            if (name === 'username') {
                const trimmedValue = value.trim();
                const error = validateUsername(trimmedValue);
                setErrors(prev => ({ ...prev, username: error }));
            } else if (name === 'email') {
                const trimmedValue = value.trim();
                const error = validateEmail(trimmedValue);
                setErrors(prev => ({ ...prev, email: error }));
            } else if (name === 'password') {
                const error = validatePassword(value);
                setErrors(prev => ({ ...prev, password: error }));
            } else if (name === 'confirmPassword') {
                setErrors(prev => ({
                    ...prev,
                    confirmPassword: value !== formData.password ? 'Passwords do not match' : ''
                }));
            }
        }
    };

    const handleBlur = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { name, value } = e.target;
        setTouched(prev => ({ ...prev, [name]: true }));

        if (name === 'username') {
            const trimmedValue = value.trim();
            if (trimmedValue !== value) {
                setFormData(prev => ({
                    ...prev,
                    [name]: trimmedValue
                }));
            }
            const error = validateUsername(trimmedValue);
            setErrors(prev => ({ ...prev, username: error }));
        } else if (name === 'email') {
            const trimmedValue = value.trim();
            if (trimmedValue !== value) {
                setFormData(prev => ({
                    ...prev,
                    [name]: trimmedValue
                }));
            }
            const error = validateEmail(trimmedValue);
            setErrors(prev => ({ ...prev, email: error }));
        } else if (name === 'password') {
            const error = validatePassword(value);
            setErrors(prev => ({ ...prev, password: error }));
        } else if (name === 'confirmPassword') {
            setErrors(prev => ({
                ...prev,
                confirmPassword: value !== formData.password ? 'Passwords do not match' : ''
            }));
        }
    };

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.currentTarget.name === 'username' && e.key === ' ') {
            e.preventDefault();
        }
    };

    const handleGoogleSignup = () => {
        if (Environment !== "DEVELOPMENT") {
            setShowGoogleSignupSecret(true);
        } else {
            initiateGoogleOAuth('auth', undefined, 'signup');
        }
    };

    const handleValidateSecret = async () => {
        if (!googleSignupSecret.trim()) {
            setSecretValidationStatus('invalid');
            return;
        }

        setIsValidatingSecret(true);
        setSecretValidationStatus(null);
        
        try {
            const isValid = await authService.validateSignupSecret(googleSignupSecret);
            setSecretValidationStatus(isValid ? 'valid' : 'invalid');
            
            if (isValid) {
                // Wait a moment for the user to see the success state
                setTimeout(() => {
                    initiateGoogleOAuth('auth', googleSignupSecret, 'signup');
                }, 500);
            }
        } catch (error: any) {
            setSecretValidationStatus('invalid');
            // setFormError(error.message || 'Failed to validate signup secret');
        } finally {
            setIsValidatingSecret(false);
        }
    };

    const handleCancelGoogleSignup = () => {
        setShowGoogleSignupSecret(false);
        setGoogleSignupSecret('');
        setSecretValidationStatus(null);
        setFormError(null);
    };

    return (
        <div className="min-h-screen bg-[#FFDB58]/20 flex items-center justify-center p-4">
            <div className="w-full max-w-md neo-border bg-white p-4 md:p-8">
                <h1 className="text-2xl md:text-3xl font-bold text-center mb-4 flex items-center justify-center gap-2">
                    <Boxes className="w-10 h-10" />
                    NeoBase
                </h1>
                <p className="text-gray-600 text-center mb-8">
                    Create your account to start using NeoBase - Your AI Database Copilot
                </p>

                {formError && (
                    <div className="mb-6 p-4 bg-red-50 border-2 border-red-500 rounded-lg">
                        <div className="flex items-center gap-2 text-red-600">
                            <AlertCircle className="w-5 h-5" />
                            <p className="font-medium">{formError}</p>
                        </div>
                    </div>
                )}

                {/* Google OAuth Button or Secret Input - Top Section */}
                {!showGoogleSignupSecret ? (
                    <button
                        type="button"
                        onClick={handleGoogleSignup}
                        className="neo-button-secondary w-full flex items-center justify-center gap-2 mb-6"
                        disabled={isLoading}
                    >
                        <svg className="w-5 h-5" viewBox="0 0 24 24">
                            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                            <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                            <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                        </svg>
                        Continue with Google
                    </button>
                ) : null}

                {/* Production-only: Show Signup Secret field under Google button */}
                {Environment !== "DEVELOPMENT" && showGoogleSignupSecret && (
                    <div className="space-y-3 mb-6">
                        <p className="text-sm font-medium text-gray-600">
                            Enter your signup secret to continue with Google
                        </p>
                        <div className="relative">
                            <Lock className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="text"
                                placeholder="Enter Signup Secret"
                                value={googleSignupSecret}
                                onChange={(e) => {
                                    setGoogleSignupSecret(e.target.value);
                                    setSecretValidationStatus(null);
                                }}
                                className="neo-input pl-12 w-full"
                                disabled={isValidatingSecret || secretValidationStatus === 'valid'}
                            />
                        </div>
                        {secretValidationStatus === 'invalid' && (
                            <div className="flex items-center gap-1 text-red-600 text-sm">
                                <AlertCircle className="w-4 h-4" />
                                <span>Invalid signup secret. Please try again.</span>
                            </div>
                        )}
                        {secretValidationStatus === 'valid' && (
                            <div className="flex items-center gap-1 text-green-600 text-sm">
                                <Check className="w-4 h-4" />
                                <span>Valid! You can now sign up with Google.</span>
                            </div>
                        )}
                        <div className="flex gap-2">
                            <button
                                type="button"
                                onClick={handleCancelGoogleSignup}
                                className="neo-button-secondary flex-1"
                                disabled={isValidatingSecret || secretValidationStatus === 'valid'}
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleValidateSecret}
                                className="neo-button flex-1"
                                disabled={isValidatingSecret || !googleSignupSecret.trim() || secretValidationStatus === 'valid'}
                            >
                                {isValidatingSecret ? (
                                    <div className="flex items-center justify-center gap-2">
                                        <Loader className="w-4 h-4 animate-spin" />
                                        Validating
                                    </div>
                                ) : (
                                    'Validate'
                                )}
                            </button>
                        </div>
                    </div>
                )}

                {/* Divider */}
                <div className="relative my-6">
                    <div className="absolute inset-0 flex items-center">
                        <div className="w-full border-t-2 border-gray-300"></div>
                    </div>
                    <div className="relative flex justify-center text-sm">
                        <span className="px-2 bg-white text-gray-500">OR</span>
                    </div>
                </div>

                <form onSubmit={handleSubmit} className="space-y-6">
                    <div>
                        <div className="relative">
                            <UserRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="text"
                                name="username"
                                placeholder="Username"
                                value={formData.username}
                                onChange={handleChange}
                                onBlur={handleBlur}
                                onKeyDown={handleKeyDown}
                                className={`neo-input pl-12 w-full ${errors.username && touched.username ? 'border-neo-error' : ''
                                    }`}
                                required
                            />
                        </div>
                        {errors.username && touched.username && (
                            <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                                <AlertCircle className="w-4 h-4" />
                                <span>{errors.username}</span>
                            </div>
                        )}
                    </div>

                    <div>
                        <div className="relative">
                            <Mail className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="email"
                                name="email"
                                placeholder="Email"
                                value={formData.email}
                                onChange={handleChange}
                                onBlur={handleBlur}
                                className={`neo-input pl-12 w-full ${errors.email && touched.email ? 'border-neo-error' : ''
                                    }`}
                                required
                            />
                        </div>
                        {errors.email && touched.email && (
                            <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                                <AlertCircle className="w-4 h-4" />
                                <span>{errors.email}</span>
                            </div>
                        )}
                    </div>

                    {Environment !== "DEVELOPMENT" && (
                        <div>
                            <div className="relative">
                                <Lock className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                                <input
                                    type="text"
                                    name="userSignupSecret"
                                    placeholder="User Signup Secret"
                                    value={formData.userSignupSecret}
                                    onChange={handleChange}
                                    onBlur={handleBlur}
                                    className={`neo-input pl-12 w-full ${errors.userSignupSecret && touched.userSignupSecret ? 'border-neo-error' : ''
                                        }`}
                                    required
                                />
                            </div>
                            <p className="text-gray-500 text-sm mt-2">
                                Required to signup a user, ask the admin for this secret.
                            </p>
                            {errors.userSignupSecret && touched.userSignupSecret && (
                                <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                                    <AlertCircle className="w-4 h-4" />
                                    <span>{errors.userSignupSecret}</span>
                                </div>
                            )}
                        </div>
                    )}

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

                    <div>
                        <div className="relative">
                            <KeyRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="password"
                                name="confirmPassword"
                                placeholder="Confirm Password"
                                value={formData.confirmPassword}
                                onChange={handleChange}
                                onBlur={handleBlur}
                                className={`neo-input pl-12 w-full ${errors.confirmPassword && touched.confirmPassword ? 'border-neo-error' : ''
                                    }`}
                                required
                            />
                        </div>
                        {errors.confirmPassword && touched.confirmPassword && (
                            <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                                <AlertCircle className="w-4 h-4" />
                                <span>{errors.confirmPassword}</span>
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
                                Signing up...
                            </div>
                        ) : (
                            'Sign Up'
                        )}
                    </button>

                    <div className="my-2" />
                    <button
                        type="button"
                        onClick={onSwitchToLogin}
                        className="neo-button-secondary w-full"
                        disabled={isLoading}
                    >
                       Have an Account - Login
                    </button>
                </form>
            </div>
        </div>
    );
}