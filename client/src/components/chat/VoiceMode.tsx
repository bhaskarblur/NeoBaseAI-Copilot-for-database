import { X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

// TypeScript declarations for Web Speech API
interface SpeechRecognitionEvent {
    results: SpeechRecognitionResultList;
    resultIndex: number;
}

interface SpeechRecognitionErrorEvent {
    error: string;
    message: string;
}

interface SpeechRecognition extends EventTarget {
    continuous: boolean;
    interimResults: boolean;
    lang: string;
    maxAlternatives?: number;
    start(): void;
    stop(): void;
    abort?(): void;
    onstart: (() => void) | null;
    onend: (() => void) | null;
    onresult: ((event: SpeechRecognitionEvent) => void) | null;
    onerror: ((event: SpeechRecognitionErrorEvent) => void) | null;
    onnomatch?: (() => void) | null;
    onsoundstart?: (() => void) | null;
    onsoundend?: (() => void) | null;
    onspeechstart?: (() => void) | null;
    onspeechend?: (() => void) | null;
}

declare global {
    interface Window {
        SpeechRecognition: {
            new(): SpeechRecognition;
        };
        webkitSpeechRecognition: {
            new(): SpeechRecognition;
        };
    }
}

interface VoiceModeProps {
    isOpen: boolean;
    onClose: () => void;
    onSendMessage: (message: string) => Promise<void>;
    voiceSteps?: string[];
    currentVoiceStep?: string;
    onResetVoiceSteps?: () => void;
    resetOnCancelKey?: number;
}

export default function VoiceMode({
    isOpen,
    onClose,
    onSendMessage,
    voiceSteps = [],
    currentVoiceStep = '',
    onResetVoiceSteps,
    resetOnCancelKey
}: VoiceModeProps) {
    const [isListening, setIsListening] = useState(false);
    const [transcript, setTranscript] = useState('');
    const [isProcessingVoice, setIsProcessingVoice] = useState(false);
    const [showVoiceResponse, setShowVoiceResponse] = useState(false);
    const [showResponseReceived, setShowResponseReceived] = useState(false);
    const [hasError, setHasError] = useState(false);
    const [errorMessage, setErrorMessage] = useState('');
    const [shouldListen, setShouldListen] = useState(true); // Controls when to accept speech input
    const shouldListenRef = useRef(true);
    const recognitionRef = useRef<SpeechRecognition | null>(null);
    const transcriptRef = useRef('');
    const silenceTimeoutRef = useRef<NodeJS.Timeout | null>(null);
    const supportRetryRef = useRef(0);
    // Robust start/retry helpers
    const isStartingRef = useRef(false);
    const isProcessingRef = useRef(false);
    const hasStepsRef = useRef(false);
    const showReceivedRef = useRef(false);
    const showCompletedRef = useRef(false);
    const isOpenRef = useRef(false);
    const closingRef = useRef(false);
    const ignoreNextOnEndRef = useRef(false);
    const lastSentRef = useRef('');
    const [isSpeechSupported, setIsSpeechSupported] = useState(false);
    const [micPermission, setMicPermission] = useState<'granted' | 'denied' | 'prompt' | 'checking'>('checking');
    const messageSentTimeoutRef = useRef<NodeJS.Timeout | null>(null);
    const lastActivityTimeRef = useRef(Date.now());

    // Play notification sound
    const playNotificationSound = () => {
        try {
            // Create a simple beep sound
            const audioContext = new (window.AudioContext || (window as any).webkitAudioContext)();
            const oscillator = audioContext.createOscillator();
            const gainNode = audioContext.createGain();
            
            oscillator.connect(gainNode);
            gainNode.connect(audioContext.destination);
            
            oscillator.frequency.value = 800; // Higher frequency beep
            oscillator.type = 'sine';
            
            gainNode.gain.setValueAtTime(0.3, audioContext.currentTime);
            gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.2);
            
            oscillator.start(audioContext.currentTime);
            oscillator.stop(audioContext.currentTime + 0.2);
        } catch (error) {
            console.log('Could not play notification sound:', error);
        }
    };

    // Check speech recognition support and request microphone permissions
    useEffect(() => {
        const requestMicrophoneAccess = async () => {
            console.log('Requesting microphone access...');
            isOpenRef.current = isOpen;
            
            if (!('webkitSpeechRecognition' in window || 'SpeechRecognition' in window)) {
                setIsSpeechSupported(false);
                setMicPermission('denied');
                return;
            }
            
            setIsSpeechSupported(true);
            
            if (!isOpen) return;
            
            try {
                // Skip permission query check on iOS Chrome as it may not be reliable
                const userAgent = navigator.userAgent;
                const isIOSChrome = /iPhone|iPad|iPod/i.test(userAgent) && (/CriOS|Chrome/i.test(userAgent));
                
                if (!isIOSChrome) {
                    // First check if permissions are already granted (not on iOS Chrome)
                    const permission = await navigator.permissions.query({ name: 'microphone' as PermissionName });
                    console.log('Current mic permission:', permission.state);
                    
                    if (permission.state === 'granted') {
                        setMicPermission('granted');
                        if (isOpenRef.current && !closingRef.current) {
                            initializeSpeechRecognition(true); // Pass permission state directly
                        }
                        return;
                    }
                }
                
                // Request microphone access with basic audio settings
                console.log('Requesting microphone stream...');
                
                // Use different audio constraints for iOS Chrome
                let audioConstraints: MediaTrackConstraints = {
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true,
                    sampleRate: 16000
                };
                
                // Simplify constraints for iOS Chrome
                if (isIOSChrome) {
                    audioConstraints = { echoCancellation: true };
                }
                
                const stream = await navigator.mediaDevices.getUserMedia({ 
                    audio: audioConstraints
                });
                
                console.log('Microphone stream obtained:', stream);
                
                // Test that we can actually get audio
                const audioTracks = stream.getAudioTracks();
                if (audioTracks.length === 0) {
                    throw new Error('No audio tracks available');
                }
                
                console.log('Audio tracks found:', audioTracks.length);
                
                // For iOS Chrome, also check track state
                if (isIOSChrome) {
                    const track = audioTracks[0];
                    console.log('Track state:', track.readyState, 'Enabled:', track.enabled);
                    if (track.readyState !== 'live') {
                        throw new Error('Audio track not active');
                    }
                }
                
                // Stop the stream immediately - we just needed to test access
                stream.getTracks().forEach(track => {
                    console.log('Stopping track:', track.label);
                    track.stop();
                });
                
                setMicPermission('granted');
                
                // Wait a bit longer for iOS Chrome before initializing speech recognition
                const initDelay = isIOSChrome ? 1000 : 500;
                setTimeout(() => {
                    console.log('Initializing speech recognition...');
                    if (isOpenRef.current && !closingRef.current) {
                        initializeSpeechRecognition(true); // Pass permission state directly
                    }
                }, initDelay);
                
            } catch (error) {
                console.error('Microphone access failed:', error);
                setMicPermission('denied');
            }
        };
        
        if (isOpen) {
            requestMicrophoneAccess();
        }
    }, [isOpen]);

    // Initialize speech recognition with improved error handling
    const initializeSpeechRecognition = (hasPermission = false) => {
        console.log('initializeSpeechRecognition called, hasPermission:', hasPermission);
        // Check raw support to avoid stale state races
        const hasSupport = ('webkitSpeechRecognition' in window) || ('SpeechRecognition' in window);
        if (!hasSupport) {
            console.log('Speech recognition not supported');
            // Retry a few times as browsers may hydrate APIs slightly later
            supportRetryRef.current += 1;
            if (supportRetryRef.current <= 3) {
                setTimeout(() => initializeSpeechRecognition(hasPermission), 300);
            } else {
                setIsSpeechSupported(false);
                setErrorMessage('Speech recognition not supported in this browser.');
            }
            return;
        } else if (!isSpeechSupported) {
            setIsSpeechSupported(true);
        }
        
        if (!hasPermission && micPermission !== 'granted') {
            console.log('Microphone permission not granted:', micPermission);
            return;
        }

        // Check if recognition exists and is working
        if (recognitionRef.current) {
            console.log('Recognition already exists - checking if it needs to be started');
            // If recognition exists but is already listening or starting, do nothing
            if (isListening || isStartingRef.current) {
                console.log('Recognition is already active/starting; skipping start');
                return;
            }
            // Try to start only if idle
            try {
                console.log('Starting existing recognition...');
                isStartingRef.current = true;
                recognitionRef.current.start();
                playNotificationSound();
                return; // Exit after starting existing recognition
            } catch (error: any) {
                // If already started, ignore
                const msg = (error && (error.message || '')) as string;
                if (msg.includes('already started')) {
                    console.warn('Start called while already started; ignoring');
                    isStartingRef.current = false;
                    return;
                }
                console.error('Failed to start existing recognition:', error);
                // For other errors, attempt to recreate
                console.log('Recreating recognition due to error...');
                recognitionRef.current = null;
                // Continue to create new recognition below
            }
        }

        const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
        
        try {
            const recognition = new SpeechRecognition();
            
            console.log('Configuring speech recognition...');
            recognition.continuous = true;
            recognition.interimResults = true;
            recognition.lang = navigator.language || 'en-US';
            if ('maxAlternatives' in recognition) {
                recognition.maxAlternatives = 1;
            }
            
            // Mobile-specific configurations
            const userAgent = navigator.userAgent.toLowerCase();
            const isIOS = /iphone|ipad|ipod/.test(userAgent);
            const isAndroid = /android/.test(userAgent);
            const isSamsung = /samsung/.test(userAgent);
            
            if (isIOS || isAndroid) {
                console.log('Detected mobile device, applying mobile-specific settings');
                // Disable continuous mode on iOS to prevent permission issues
                if (isIOS) {
                    recognition.continuous = false;
                    console.log('iOS device detected: disabled continuous mode');
                }
                
                // Samsung-specific optimizations
                if (isSamsung || isAndroid) {
                    console.log('Android/Samsung device detected: applying specific settings');
                    // Reduce interim results on Android to improve accuracy
                    recognition.interimResults = false;
                }
            }
        
            recognition.onstart = () => {
                console.log('Speech recognition started successfully');
                setIsListening(true);
                setTranscript('');
                transcriptRef.current = '';
                playNotificationSound(); // Play sound when listening starts
                isStartingRef.current = false;
            };
        
            recognition.onresult = (event: SpeechRecognitionEvent) => {
                // Ignore speech input if we shouldn't be listening (during processing)
                if (!isOpenRef.current || !shouldListenRef.current || isProcessingRef.current || hasStepsRef.current || showReceivedRef.current || showCompletedRef.current) {
                    console.log('Ignoring speech input - currently processing');
                    return;
                }

                let finalTranscript = '';
                let interimTranscript = '';
                
                for (let i = event.resultIndex; i < event.results.length; i++) {
                    const transcript = event.results[i][0].transcript;
                    if (event.results[i].isFinal) {
                        finalTranscript += transcript;
                    } else {
                        interimTranscript += transcript;
                    }
                }
                
                const fullTranscript = finalTranscript + interimTranscript;
                setTranscript(fullTranscript);
                transcriptRef.current = fullTranscript;
                
                console.log('Speech detected:', fullTranscript);
                
                // Reset silence timeout whenever we get speech
                if (silenceTimeoutRef.current) {
                    clearTimeout(silenceTimeoutRef.current);
                    silenceTimeoutRef.current = null;
                }
                
                // If we have a final transcript, send it immediately
                if (finalTranscript.trim()) {
                    console.log('Final transcript received, sending immediately:', finalTranscript.trim());
                    if (silenceTimeoutRef.current) {
                        clearTimeout(silenceTimeoutRef.current);
                        silenceTimeoutRef.current = null;
                    }
                    
                    // For mobile devices, add debounce to prevent double triggers
                    const userAgent = navigator.userAgent.toLowerCase();
                    const isMobile = /iphone|ipad|ipod|android/.test(userAgent);
                    const isSamsung = /samsung/.test(userAgent);
                    const sendDelay = isSamsung ? 500 : (isMobile ? 300 : 0);
                    
                    // Clear any existing message timeout
                    if (messageSentTimeoutRef.current) {
                        clearTimeout(messageSentTimeoutRef.current);
                    }
                    
                    // Debounce message sending
                    messageSentTimeoutRef.current = setTimeout(() => {
                        // Check if we haven't already sent this message
                        if (lastSentRef.current === finalTranscript.trim()) {
                            console.log('Duplicate message prevented');
                            return;
                        }
                        
                        // Check if enough time has passed since last activity (for Samsung double-click issue)
                        const timeSinceLastActivity = Date.now() - lastActivityTimeRef.current;
                        if (isSamsung && timeSinceLastActivity < 1000) {
                            console.log('Samsung: Ignoring rapid fire transcript');
                            return;
                        }
                        
                        // Minimum transcript length for mobile devices to avoid accidental triggers
                        const minLength = isMobile ? 3 : 1;
                        if (finalTranscript.trim().length < minLength) {
                            console.log('Transcript too short, ignoring');
                            return;
                        }
                        
                        lastActivityTimeRef.current = Date.now();
                        
                        // Do NOT stop recognition; just pause processing and send
                        setShouldListen(false);
                        shouldListenRef.current = false;
                        ignoreNextOnEndRef.current = true; // suppress immediate onend fallback
                        lastSentRef.current = finalTranscript.trim();
                        handleVoiceMessage(finalTranscript.trim());
                        // We'll resume after response completed path
                    }, sendDelay);
                }
            };
        
            recognition.onerror = (event: SpeechRecognitionErrorEvent) => {
                console.error('Speech recognition error:', event.error, event.message);
                setIsListening(false);
                
                if (event.error === 'not-allowed' || event.error === 'service-not-allowed') {
                    setMicPermission('denied');
                    
                    // Browser and platform specific handling
                    const userAgent = navigator.userAgent;
                    const isIOS = /iPhone|iPad|iPod/i.test(userAgent);
                    const isChrome = /Chrome/i.test(userAgent) && !/EdgA|OPR/i.test(userAgent);
                    const isSafari = /Safari/i.test(userAgent) && !/Chrome|CriOS|EdgA|OPR/i.test(userAgent);
                    const isFirefox = /Firefox/i.test(userAgent);
                    const isEdge = /EdgA|Edge/i.test(userAgent);
                    
                    if (isIOS) {
                        if (isChrome || /CriOS/i.test(userAgent)) {
                            setErrorMessage('Please allow microphone access in Chrome. Tap the microphone icon in the address bar or go to Settings > Privacy & Security > Site Settings > Microphone.');
                        } else if (isSafari) {
                            setErrorMessage('Please allow microphone access in Settings > Safari > Microphone.');
                        } else if (isFirefox || /FxiOS/i.test(userAgent)) {
                            setErrorMessage('Please allow microphone access in Firefox settings or try refreshing the page.');
                        } else if (isEdge || /EdgiOS/i.test(userAgent)) {
                            setErrorMessage('Please allow microphone access in Edge settings or try refreshing the page.');
                        } else {
                            setErrorMessage('Please allow microphone access in your browser settings or try refreshing the page.');
                        }
                    } else {
                        setErrorMessage('Microphone access denied. Please enable it in your browser settings.');
                    }
                } else if (event.error === 'no-speech') {
                    // Restart recognition automatically after no-speech timeout (only if should be listening)
                    if (shouldListen) {
                        console.log('No speech detected, restarting recognition...');
                        setTimeout(() => {
                            if (recognitionRef.current && !isListening && shouldListen) {
                                try {
                                    isStartingRef.current = true;
                                    recognitionRef.current.start();
                                } catch (error: any) {
                                    const msg = (error && (error.message || '')) as string;
                                    if (msg.includes('already started')) {
                                        console.warn('Restart called while already started; ignoring');
                                    } else {
                                        console.error('Failed to restart recognition:', error);
                                    }
                                    isStartingRef.current = false;
                                }
                            }
                        }, 600);
                    } else {
                        console.log('No speech detected but not currently listening for input - no restart needed');
                    }
                } else if (event.error === 'network') {
                    setErrorMessage('Network error. Please check your connection.');
                } else if (event.error === 'audio-capture') {
                    setErrorMessage('Audio capture failed. Please check your microphone.');
                } else if (event.error === 'aborted') {
                    // Silently handle aborted errors (common on mobile)
                    console.log('Recognition aborted, will restart');
                } else {
                    setErrorMessage(`Voice recognition error: ${event.error}. Please try again.`);
                }
            };
            
            recognition.onend = () => {
                console.log('Speech recognition ended');
                setIsListening(false);
                
                // Only process transcript if we should be listening (and not processing/stepping)
                if (ignoreNextOnEndRef.current) {
                    // Suppress immediate onend triggered by our own send flow
                    ignoreNextOnEndRef.current = false;
                    return;
                }
                if (!shouldListen || !isOpen || closingRef.current || isProcessingVoice || showResponseReceived || showVoiceResponse || voiceSteps.length > 0) {
                    console.log('Recognition ended but not listening for input - ignoring transcript');
                    return;
                }
                
                // Use transcriptRef to get the latest transcript value
                const currentTranscript = transcriptRef.current.trim();
                console.log('Transcript on end:', currentTranscript);
                if (currentTranscript) {
                    if (currentTranscript === lastSentRef.current) {
                        console.log('Ignoring duplicate transcript on onend');
                        return;
                    }
                    console.log('Sending voice message:', currentTranscript);
                    // Since we no longer stop on final, onend shouldn't send; but keep as fallback
                    handleVoiceMessage(currentTranscript);
                } else {
                    console.log('No transcript to send');
                    // If onend fired without transcript, re-start safely
                    setTimeout(() => {
                        if (shouldListen && isOpen && !closingRef.current) {
                            if (recognitionRef.current) {
                                try {
                                    recognitionRef.current.start();
                                } catch (e) {
                                    console.warn('Restart after onend no-transcript failed', e);
                                }
                            } else {
                                initializeSpeechRecognition(true);
                            }
                        }
                    }, 250);
                }
            };
            
            recognitionRef.current = recognition;
            
            // Start listening immediately with better error handling
            setTimeout(() => {
                if (recognition && (hasPermission || micPermission === 'granted') && !isListening && !isStartingRef.current) {
                    try {
                        console.log('Starting new speech recognition...');
                        isStartingRef.current = true;
                        recognition.start();
                    } catch (error: any) {
                        const msg = (error && (error.message || '')) as string;
                        if (msg.includes('already started')) {
                            console.warn('New recognition start raced; ignoring');
                        } else {
                            console.error('Failed to start speech recognition:', error);
                            setErrorMessage('Failed to start voice recognition. Please try again.');
                        }
                        isStartingRef.current = false;
                    }
                }
            }, 100);
            
        } catch (error) {
            console.error('Failed to initialize speech recognition:', error);
            setErrorMessage('Failed to initialize voice recognition.');
        }
    };

    // Removed unused function startListening - using initializeSpeechRecognition directly

    const handleVoiceMessage = async (voiceText: string) => {
        console.log('Processing voice message, pausing listening...');
        
        // Clear any previous timeouts
        if (silenceTimeoutRef.current) {
            clearTimeout(silenceTimeoutRef.current);
            silenceTimeoutRef.current = null;
        }
        
        setShouldListen(false); // Pause accepting speech input
        setIsProcessingVoice(true);
        setTranscript('');
        transcriptRef.current = '';
        setShowVoiceResponse(false);
        setShowResponseReceived(false);
        setHasError(false);
        setErrorMessage('');
        
        try {
            await onSendMessage(voiceText);
        } catch (error) {
            console.error('Error sending voice message:', error);
            setErrorMessage('Failed to send voice message');
        } finally {
            setIsProcessingVoice(false);
        }
    };

    // Handle SSE state changes
    useEffect(() => {
        if (currentVoiceStep && currentVoiceStep.includes('AI response received!')) {
            // Show the "AI response received!" intermediate step
            setShowResponseReceived(true);
            showReceivedRef.current = true;
            setShowVoiceResponse(false);
            setHasError(false);
            setErrorMessage('');
            setIsProcessingVoice(false);
            isProcessingRef.current = false;
            
            // After 1.5 seconds, transition to ready state
            setTimeout(() => {
                setShowResponseReceived(false);
                showReceivedRef.current = false;
                setShowVoiceResponse(true);
                showCompletedRef.current = true;
                // Auto restart listening after showing completion
                setTimeout(() => {
                    setShowVoiceResponse(false);
                    showCompletedRef.current = false;
                    console.log('Resuming voice listening...');
                    // Resume listening instead of reinitializing
                    setShouldListen(true);
                    shouldListenRef.current = true;
                    setTranscript('');
                    transcriptRef.current = '';
                    // Resume actively if needed
                    setTimeout(() => {
                        if (isOpenRef.current && !closingRef.current) {
                            if (recognitionRef.current) {
                                try { recognitionRef.current.start(); } catch {}
                            } else {
                                initializeSpeechRecognition(true);
                            }
                        }
                    }, 100);
                    // Restart recognition if it's not running
                    if (recognitionRef.current && !isListening) {
                        try {
                            console.log('Restarting existing recognition...');
                            // For iOS, recreate recognition instead of restarting
                            if (/iPhone|iPad|iPod/i.test(navigator.userAgent)) {
                                recognitionRef.current = null;
                                initializeSpeechRecognition(true);
                            } else {
                                recognitionRef.current.start();
                                playNotificationSound();
                            }
                        } catch (error) {
                            console.error('Failed to restart recognition:', error);
                            // Just log the error, don't recreate - mic should stay persistent
                            setErrorMessage('Failed to restart voice recognition. Please try again.');
                        }
                    } else if (!recognitionRef.current) {
                        console.log('No recognition object - this should not happen in persistent mode');
                        setErrorMessage('Voice recognition lost. Please close and reopen voice mode.');
                    }
                }, 2000);
            }, 1500);
        } else if (currentVoiceStep && currentVoiceStep.includes('Response ready')) {
            setShowVoiceResponse(true);
            setHasError(false);
            setErrorMessage('');
            setIsProcessingVoice(false);
        } else if (currentVoiceStep && (currentVoiceStep.includes('error') || currentVoiceStep.includes('Error') || currentVoiceStep.includes('failed') || currentVoiceStep.includes('Failed'))) {
            // Handle error states
            setHasError(true);
            setErrorMessage(currentVoiceStep);
            setIsProcessingVoice(false);
            isProcessingRef.current = false;
            setShowVoiceResponse(false);
            setShowResponseReceived(false);
            // Auto-clear error after 1s and resume listening
            setTimeout(() => {
                setHasError(false);
                setErrorMessage('');
                setTranscript('');
                transcriptRef.current = '';
                setShouldListen(true);
                shouldListenRef.current = true;
                if (recognitionRef.current && !isListening) {
                    try {
                        recognitionRef.current.start();
                    } catch (e) {
                        console.error('Failed to restart after error:', e);
                    }
                }
            }, 1000);
        }
    }, [currentVoiceStep]);

    // If error state comes via voiceSteps (fallback), auto-clear in 1s too
    useEffect(() => {
        if (hasError) {
            const t = setTimeout(() => {
                setHasError(false);
                setErrorMessage('');
                setTranscript('');
                transcriptRef.current = '';
                setShouldListen(true);
                if (recognitionRef.current && !isListening) {
                    try {
                        recognitionRef.current.start();
                    } catch (e) {
                        console.error('Failed to restart after error (fallback):', e);
                    }
                }
            }, 1000);
            return () => clearTimeout(t);
        }
    }, [hasError]);

    // Handle ai-response-error events from voiceSteps
    useEffect(() => {
        if (voiceSteps.length > 0) {
            const lastStep = voiceSteps[voiceSteps.length - 1];
            if (lastStep && (lastStep.includes('error') || lastStep.includes('Error') || lastStep.includes('failed') || lastStep.includes('Failed'))) {
                // Handle error from voiceSteps
                setHasError(true);
                setErrorMessage(lastStep);
                setIsProcessingVoice(false);
                isProcessingRef.current = false;
                setShowVoiceResponse(false);
                setShowResponseReceived(false);
            }
        }
    }, [voiceSteps]);

    // Sync busy/listening flags to refs so gating logic reflects latest state
    useEffect(() => {
        shouldListenRef.current = shouldListen;
    }, [shouldListen]);

    useEffect(() => {
        isProcessingRef.current = isProcessingVoice;
    }, [isProcessingVoice]);

    useEffect(() => {
        hasStepsRef.current = voiceSteps.length > 0;
    }, [voiceSteps]);

    useEffect(() => {
        showReceivedRef.current = showResponseReceived;
    }, [showResponseReceived]);

    useEffect(() => {
        showCompletedRef.current = showVoiceResponse;
    }, [showVoiceResponse]);

    // Reset all voice states
    const resetVoiceStates = () => {
        setTranscript('');
        setIsProcessingVoice(false);
        setShowVoiceResponse(false);
        setShowResponseReceived(false);
        setHasError(false);
        setErrorMessage('');
        setIsListening(false);
        setShouldListen(true); // Reset to listening state
        transcriptRef.current = '';
    };

    // Cleanup on close and reset on open
    useEffect(() => {
        if (!isOpen) {
            closingRef.current = true;
            if (recognitionRef.current && isListening) {
                recognitionRef.current.stop();
            }
            if (silenceTimeoutRef.current) {
                clearTimeout(silenceTimeoutRef.current);
            }
            // Clear any restart timers / flags and release mic
            isStartingRef.current = false;
            transcriptRef.current = '';
            recognitionRef.current = null;
            shouldListenRef.current = false;
            isProcessingRef.current = false;
            hasStepsRef.current = false;
            showReceivedRef.current = false;
            showCompletedRef.current = false;
            isOpenRef.current = false;
            resetVoiceStates();
            setMicPermission('checking');
        } else {
            // Reset states when opening voice mode
            resetVoiceStates();
            shouldListenRef.current = true;
            isOpenRef.current = true;
            closingRef.current = false;
            // Clear external voice steps too
            if (onResetVoiceSteps) {
                onResetVoiceSteps();
            }
        }
    }, [isOpen]);

    // When a cancel occurs upstream, reset the UI to default idle listening
    useEffect(() => {
        if (!isOpen) return;
        // Reset local UI state to idle and ensure listening resumes
        setHasError(false);
        setErrorMessage('');
        setShowResponseReceived(false);
        setShowVoiceResponse(false);
        setIsProcessingVoice(false);
        isProcessingRef.current = false;
        setTranscript('');
        transcriptRef.current = '';
        setShouldListen(true);
        shouldListenRef.current = true;
        // Make sure recognition is active only if we're not mid-steps
        if (voiceSteps.length === 0) {
            setTimeout(() => {
                if (recognitionRef.current) {
                    try { recognitionRef.current.start(); } catch {}
                } else {
                    initializeSpeechRecognition(true);
                }
            }, 100);
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [resetOnCancelKey]);

    // Safety: if in idle state with permission granted, ensure listening and green waves
    useEffect(() => {
        if (!isOpen) return;
        const inIdle = !isProcessingVoice && !hasError && !showResponseReceived && !showVoiceResponse && voiceSteps.length === 0;
        if (inIdle && micPermission === 'granted') {
            if (!shouldListen) setShouldListen(true);
            setTimeout(() => {
                if (recognitionRef.current) {
                    try { recognitionRef.current.start(); } catch {}
                } else {
                    initializeSpeechRecognition(true);
                }
            }, 50);
        }
    }, [isOpen, isProcessingVoice, hasError, showResponseReceived, showVoiceResponse, micPermission, shouldListen, voiceSteps.length]);

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            if (recognitionRef.current) {
                recognitionRef.current.stop();
            }
            if (silenceTimeoutRef.current) {
                clearTimeout(silenceTimeoutRef.current);
            }
            if (messageSentTimeoutRef.current) {
                clearTimeout(messageSentTimeoutRef.current);
            }
        };
    }, []);

    if (!isOpen) return null;

    // Error states
    if (!isSpeechSupported || micPermission === 'denied') {
        return (
            <div className="bg-white border-4 border-black rounded-2xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] p-4 flex flex-col overflow-hidden h-20">
                <div className={`transition-all duration-300 ease-out delay-150 ${
                    isOpen ? 'transform translate-y-0 opacity-100' : 'transform translate-y-4 opacity-0'
                }`}>
                <div className="relative flex flex-col items-center justify-center text-center h-full">
                    <button
                        onClick={onClose}
                        className="absolute top-1 right-1 p-1 hover:bg-neo-gray rounded-lg transition-colors"
                    >
                        <X className="w-4 h-4" />
                    </button>
                    <div>
                        <p className="text-base font-bold text-neo-error mb-1 -mt-5">
                            {!isSpeechSupported ? 'Speech not supported' : 'Microphone access needed'}
                        </p>
                        <p className="text-sm text-gray-600">
                            {!isSpeechSupported ? (
                                'Please use Chrome, Safari, or Edge for voice features.'
                            ) : (() => {
                                const userAgent = navigator.userAgent;
                                const isIOS = /iPhone|iPad|iPod/i.test(userAgent);
                                const isChrome = /Chrome/i.test(userAgent) && !/EdgA|OPR/i.test(userAgent);
                                const isSafari = /Safari/i.test(userAgent) && !/Chrome|CriOS|EdgA|OPR/i.test(userAgent);
                                const isFirefox = /Firefox/i.test(userAgent);
                                const isEdge = /EdgA|Edge/i.test(userAgent);
                                
                                if (isIOS) {
                                    if (isChrome || /CriOS/i.test(userAgent)) {
                                        return 'Please allow microphone access in Chrome. Tap the microphone icon in the address bar.';
                                    } else if (isSafari) {
                                        return 'Please allow microphone access in Settings > Safari > Microphone.';
                                    } else if (isFirefox || /FxiOS/i.test(userAgent)) {
                                        return 'Please allow microphone access in Firefox settings.';
                                    } else if (isEdge || /EdgiOS/i.test(userAgent)) {
                                        return 'Please allow microphone access in Edge settings.';
                                    } else {
                                        return 'Please allow microphone access in your browser settings.';
                                    }
                                }
                                return 'Please allow microphone access to use voice mode.';
                            })()
                            }
                        </p>
                    </div>
                </div>
                {micPermission === 'denied' && (
                    <div className="flex items-center justify-center h-full">
                        <div className="flex flex-row items-center justify-center gap-3">
                            <button
                                onClick={async () => {
                                    try {
                                        await navigator.mediaDevices.getUserMedia({ audio: true });
                                        setMicPermission('granted');
                                        // Force re-initialize speech recognition after permission granted
                                        setTimeout(() => {
                                            if (isOpenRef.current && !closingRef.current) {
                                                initializeSpeechRecognition(true);
                                            }
                                        }, 100);
                                    } catch (error) {
                                        console.error('Failed to get microphone permission:', error);
                                        const userAgent = navigator.userAgent;
                                        const isIOS = /iPhone|iPad|iPod/i.test(userAgent);
                                        const isChrome = /Chrome/i.test(userAgent) && !/EdgA|OPR/i.test(userAgent);
                                        
                                        if (isIOS && (isChrome || /CriOS/i.test(userAgent))) {
                                            setErrorMessage('Please tap the microphone icon in Chrome address bar and allow access.');
                                        } else {
                                            setErrorMessage('Please enable microphone in browser settings.');
                                        }
                                    }
                                }}
                                className="px-4 py-2 bg-black text-white text-sm rounded hover:bg-gray-800 transition-colors"
                            >
                                Enable Microphone
                            </button>
                        </div>
                    </div>
                )}
                </div>
            </div>
        );
    }

    return (
        <div className="bg-white border-4 border-black rounded-2xl shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] overflow-hidden h-20 relative">
            <div className={`h-full transition-all duration-300 ease-out delay-150 ${
                isOpen ? 'transform translate-y-0 opacity-100' : 'transform translate-y-4 opacity-0'
            }`}>
                {/* Header with close button - absolute positioned */}
                <div className="absolute top-2 right-2 z-10">
                    <button
                        onClick={onClose}
                        className="p-1 hover:bg-neo-gray rounded-lg transition-colors"
                        title="Exit voice mode"
                    >
                        <X className="w-4 h-4" />
                    </button>
                </div>
                {/* Main content - perfectly centered */}
                <div className="flex items-center justify-center space-x-3 h-full px-12">
                    {/* Waveform/Status indicator */}
                    {shouldListen ? (
                        <div className="flex items-end space-x-1">
                            {[...Array(6)].map((_, i) => (
                                <div
                                    key={i}
                                    className="bg-neo-success rounded-full animate-pulse"
                                    style={{
                                        width: '3px',
                                        height: `${6 + (Math.random() * 12)}px`,
                                        animationDelay: `${i * 100}ms`,
                                        animationDuration: '0.6s'
                                    }}
                                ></div>
                            ))}
                        </div>
                    ) : !shouldListen ? (
                        <div className="flex items-center space-x-1">
                            {[...Array(4)].map((_, i) => (
                                <div
                                    key={i}
                                    className="w-1 h-2 bg-yellow-400 rounded-full"
                                ></div>
                            ))}
                        </div>
                    ) : hasError ? (
                        <div className="w-3 h-3 bg-neo-error rounded-full"></div>
                    ) : isProcessingVoice || (voiceSteps.length > 0 && !showResponseReceived && !showVoiceResponse) ? (
                        <div className="flex space-x-1">
                            <div className="w-1.5 h-1.5 bg-primary-yellow rounded-full animate-bounce" style={{ animationDelay: '0ms' }}></div>
                            <div className="w-1.5 h-1.5 bg-primary-yellow rounded-full animate-bounce" style={{ animationDelay: '150ms' }}></div>
                            <div className="w-1.5 h-1.5 bg-primary-yellow rounded-full animate-bounce" style={{ animationDelay: '300ms' }}></div>
                        </div>
                    ) : showResponseReceived ? (
                        <div className="w-3 h-3 bg-primary-yellow rounded-full animate-pulse"></div>
                    ) : showVoiceResponse ? (
                        <div className="w-3 h-3 bg-neo-success rounded-full animate-pulse"></div>
                    ) : (
                        <div className="flex items-center space-x-1">
                            {[...Array(4)].map((_, i) => (
                                <div
                                    key={i}
                                    className="w-1 h-2 bg-gray-300 rounded-full"
                                ></div>
                            ))}
                        </div>
                    )}

                    {/* Transcript or status text */}
                    {hasError ? (
                        <p className="text-base text-neo-error font-medium">
                            {errorMessage}
                        </p>
                    ) : transcript ? (
                        <p className="text-base text-gray-800 font-medium">
                            {transcript}
                        </p>
                    ) : isProcessingVoice || (voiceSteps.length > 0 && !showResponseReceived && !showVoiceResponse) ? (
                        <p className={`text-gray-600 break-words ${
                            (currentVoiceStep || 'Processing...').length > 45 
                                ? 'text-sm md:text-base' 
                                : 'text-base'
                        }`}>
                            {currentVoiceStep || 'Processing...'}
                        </p>
                    ) : showResponseReceived ? (
                        <p className="text-base text-primary-yellow font-medium">
                            AI Response Received!
                        </p>
                    ) : showVoiceResponse ? (
                        <p className="text-base text-neo-success font-medium">
                            AI Response Completed
                        </p>
                    ) : (
                        <p className="text-base text-gray-500">
                            {micPermission === 'granted' ? 'You can speak now...' : 'Checking microphone...'}
                        </p>
                    )}
                </div>
            </div>
        </div>
    );
}