import { AlertCircle, Boxes, Mail, KeyRound, Loader } from 'lucide-react';
import React, { useState } from 'react';
import toast from 'react-hot-toast';

interface ResetPasswordFormProps {
    initialEmail: string;
    onSwitchToLogin: () => void;
    onPasswordResetSuccess: () => void;
}

interface FormErrors {
    email?: string;
    otp?: string;
    newPassword?: string;
    confirmPassword?: string;
}

export default function ResetPasswordForm({ initialEmail, onSwitchToLogin, onPasswordResetSuccess }: ResetPasswordFormProps) {
    const [errors, setErrors] = useState<FormErrors>({});
    const [touched, setTouched] = useState<Record<string, boolean>>({});
    const [isLoading, setIsLoading] = useState(false);
    const [formData, setFormData] = useState({
        email: initialEmail,
        otp: '',
        newPassword: '',
        confirmPassword: ''
    });
    const [formError, setFormError] = useState<string | null>(null);

    const validateEmail = (email: string) => {
        if (!email) return 'Email is required';
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        if (!emailRegex.test(email)) return 'Please enter a valid email address';
        return '';
    };

    const validateOTP = (otp: string) => {
        if (!otp) return 'OTP is required';
        if (otp.length !== 6) return 'OTP must be 6 digits';
        if (!/^\d+$/.test(otp)) return 'OTP must contain only numbers';
        return '';
    };

    const validatePassword = (password: string) => {
        if (!password) return 'Password is required';
        if (password.length < 6) return 'Password must be at least 6 characters';
        return '';
    };

    const validateForm = () => {
        const newErrors: FormErrors = {};

        const emailError = validateEmail(formData.email);
        if (emailError) newErrors.email = emailError;

        const otpError = validateOTP(formData.otp);
        if (otpError) newErrors.otp = otpError;

        const passwordError = validatePassword(formData.newPassword);
        if (passwordError) newErrors.newPassword = passwordError;

        if (formData.newPassword !== formData.confirmPassword) {
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
            const response = await fetch(`${import.meta.env.VITE_API_URL}/auth/reset-password`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    email: formData.email,
                    otp: formData.otp,
                    new_password: formData.newPassword
                }),
            });

            const data = await response.json();

            if (data.success) { 
                onPasswordResetSuccess();
            } else {
                setFormError(data.error || 'Failed to reset password');
            }
        } catch (error: any) {
            setFormError('Network error. Please try again.');
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
            if (name === 'email') {
                const trimmedValue = value.trim();
                const error = validateEmail(trimmedValue);
                setErrors(prev => ({ ...prev, email: error }));
            } else if (name === 'otp') {
                const error = validateOTP(value);
                setErrors(prev => ({ ...prev, otp: error }));
            } else if (name === 'newPassword') {
                const error = validatePassword(value);
                setErrors(prev => ({ ...prev, newPassword: error }));
            } else if (name === 'confirmPassword') {
                setErrors(prev => ({
                    ...prev,
                    confirmPassword: value !== formData.newPassword ? 'Passwords do not match' : ''
                }));
            }
        }
    };

    const handleBlur = (e: React.ChangeEvent<HTMLInputElement>) => {
        const { name, value } = e.target;
        setTouched(prev => ({ ...prev, [name]: true }));

        if (name === 'email') {
            const trimmedValue = value.trim();
            if (trimmedValue !== value) {
                setFormData(prev => ({
                    ...prev,
                    [name]: trimmedValue
                }));
            }
            const error = validateEmail(trimmedValue);
            setErrors(prev => ({ ...prev, email: error }));
        } else if (name === 'otp') {
            const error = validateOTP(value);
            setErrors(prev => ({ ...prev, otp: error }));
        } else if (name === 'newPassword') {
            const error = validatePassword(value);
            setErrors(prev => ({ ...prev, newPassword: error }));
        } else if (name === 'confirmPassword') {
            setErrors(prev => ({
                ...prev,
                confirmPassword: value !== formData.newPassword ? 'Passwords do not match' : ''
            }));
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
                    Enter the OTP sent to your email(check spam if not found) and your new password
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

                    <div>
                        <div className="relative">
                            <KeyRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="text"
                                name="otp"
                                placeholder="6-digit OTP"
                                value={formData.otp}
                                onChange={handleChange}
                                onBlur={handleBlur}
                                maxLength={6}
                                className={`neo-input pl-12 w-full ${errors.otp && touched.otp ? 'border-neo-error' : ''
                                    }`}
                                required
                            />
                        </div>
                        {errors.otp && touched.otp && (
                            <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                                <AlertCircle className="w-4 h-4" />
                                <span>{errors.otp}</span>
                            </div>
                        )}
                    </div>

                    <div>
                        <div className="relative">
                            <KeyRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="password"
                                name="newPassword"
                                placeholder="New Password"
                                value={formData.newPassword}
                                onChange={handleChange}
                                onBlur={handleBlur}
                                className={`neo-input pl-12 w-full ${errors.newPassword && touched.newPassword ? 'border-neo-error' : ''
                                    }`}
                                required
                            />
                        </div>
                        {errors.newPassword && touched.newPassword && (
                            <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                                <AlertCircle className="w-4 h-4" />
                                <span>{errors.newPassword}</span>
                            </div>
                        )}
                    </div>

                    <div>
                        <div className="relative">
                            <KeyRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                            <input
                                type="password"
                                name="confirmPassword"
                                placeholder="Confirm New Password"
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
                                Resetting Password...
                            </div>
                        ) : (
                            'Reset Password'
                        )}
                    </button>

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