import { AlertCircle, Boxes, Mail, Loader } from 'lucide-react';
import React, { useState } from 'react';

interface ForgotPasswordFormProps {
    onSwitchToLogin: () => void;
    onSwitchToResetPassword: (email: string) => void;
}

interface FormErrors {
    email?: string;
}

export default function ForgotPasswordForm({ onSwitchToLogin, onSwitchToResetPassword }: ForgotPasswordFormProps) {
    const [errors, setErrors] = useState<FormErrors>({});
    const [touched, setTouched] = useState<Record<string, boolean>>({});
    const [isLoading, setIsLoading] = useState(false);
    const [email, setEmail] = useState('');
    const [formError, setFormError] = useState<string | null>(null);
    const [successMessage, setSuccessMessage] = useState<string | null>(null);

    const validateEmail = (email: string) => {
        if (!email) return 'Email is required';
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        if (!emailRegex.test(email)) return 'Please enter a valid email address';
        return '';
    };

    const validateForm = () => {
        const newErrors: FormErrors = {};
        const emailError = validateEmail(email);
        if (emailError) newErrors.email = emailError;
        setErrors(newErrors);
        return Object.keys(newErrors).length === 0;
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setErrors({});
        setFormError(null);
        setSuccessMessage(null);

        if (!validateForm()) return;

        setIsLoading(true);
        try {
            const response = await fetch(`${import.meta.env.VITE_API_URL}/auth/forgot-password`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ email }),
            });

            const data = await response.json();

            if (data.success) {
                setSuccessMessage(data.data.message);
                // Switch to reset password form after 2 seconds
                setTimeout(() => {
                    onSwitchToResetPassword(email);
                }, 2000);
            } else {
                setFormError(data.error || 'Failed to send reset email');
            }
        } catch (error: any) {
            setFormError('Network error. Please try again.');
        } finally {
            setIsLoading(false);
        }
    };

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { value } = e.target;
        setEmail(value);

        if (touched.email) {
            const trimmedValue = value.trim();
            const error = validateEmail(trimmedValue);
            setErrors(prev => ({ ...prev, email: error }));
        }
    };

    const handleBlur = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { value } = e.target;
        setTouched(prev => ({ ...prev, email: true }));

        const trimmedValue = value.trim();
        if (trimmedValue !== value) {
            setEmail(trimmedValue);
        }
        const error = validateEmail(trimmedValue);
        setErrors(prev => ({ ...prev, email: error }));
    };

    return (
        <div className="min-h-screen bg-[#FFDB58]/20 flex items-center justify-center p-4">
            <div className="w-full max-w-md neo-border bg-white p-4 md:p-8">
                <h1 className="text-2xl md:text-3xl font-bold text-center mb-4 flex items-center justify-center gap-2">
                    <Boxes className="w-10 h-10" />
                    NeoBase
                </h1>
                <p className="text-gray-600 text-center mb-8">
                    Enter your email to receive a password reset code
                </p>

                {formError && (
                    <div className="mb-6 p-4 bg-red-50 border-2 border-red-500 rounded-lg">
                        <div className="flex items-center gap-2 text-red-600">
                            <AlertCircle className="w-5 h-5" />
                            <p className="font-medium">{formError}</p>
                        </div>
                    </div>
                )}

                {successMessage && (
                    <div className="mb-6 p-4 bg-green-50 border-2 border-green-500 rounded-lg">
                        <div className="flex items-center gap-2 text-green-600">
                            <AlertCircle className="w-5 h-5" />
                            <p className="font-medium">{successMessage}</p>
                        </div>
                    </div>
                )}

                <form onSubmit={handleSubmit} className="space-y-6">
                    <div>
                        <div className="relative">
                            <Mail className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="email"
                                name="email"
                                placeholder="Email"
                                value={email}
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

                    <button
                        type="submit"
                        className="neo-button w-full relative"
                        disabled={isLoading}
                    >
                        {isLoading ? (
                            <div className="flex items-center justify-center">
                                <Loader className="w-4 h-4 animate-spin text-gray-400 mr-2" />
                                Sending Reset Code...
                            </div>
                        ) : (
                            'Send Reset Code'
                        )}
                    </button>

                    <p className="text-gray-500 text-sm mt-4 text-center">
                        We only support forgot passwords for new users with email, old users may contact us at{' '}
                        <a
                            href="mailto:neobaseai@gmail.com"
                            className="text-green-600 hover:text-green-800 underline font-medium"
                        >
                            neobaseai@gmail.com
                        </a>
                    </p>

                    <div className="my-2" />

                    <button
                        type="button"
                        onClick={onSwitchToLogin}
                        className="neo-button-secondary w-full"
                        disabled={isLoading}
                    >
                        Back to Login
                    </button>
                </form>
            </div>
        </div>
    );
}