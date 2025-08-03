import { AlertCircle, Boxes, KeyRound, Loader, Lock, UserRound, Mail } from 'lucide-react';
import React, { useState } from 'react';
import { SignupFormData } from '../../types/auth';
import analyticsService from '../../services/analyticsService';

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
                        Switch to Login
                    </button>
                </form>
            </div>
        </div>
    );
}