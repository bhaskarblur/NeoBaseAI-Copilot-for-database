import React from 'react';

interface FloatingBackgroundProps {
  count?: number;
  opacity?: number;
}

const FloatingBackground: React.FC<FloatingBackgroundProps> = ({ 
  count = 15, 
  opacity = 0.05 
}) => {
  return (
    <div className="absolute inset-0 -z-10 overflow-hidden" style={{ opacity }}>
      {Array.from({ length: count }).map((_, i) => (
        <img 
          key={i}
          src="/neobase-logo.svg" 
          alt="" 
          className="absolute w-16 h-16 animate-float"
          style={{
            top: `${Math.random() * 100}%`,
            left: `${Math.random() * 100}%`,
            animationDelay: `${Math.random() * 10}s`,
            animationDuration: `${Math.random() * 10 + 15}s`,
            transform: `rotate(${Math.random() * 360}deg)`,
            opacity: Math.random() * 0.5 + 0.5,
          }}
        />
      ))}
    </div>
  );
};

export default FloatingBackground; 