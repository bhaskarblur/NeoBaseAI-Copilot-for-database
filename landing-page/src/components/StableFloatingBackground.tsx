import React, { useMemo } from 'react';

interface FloatingBackgroundProps {
  count?: number;
  opacity?: number;
}

const StableFloatingBackground: React.FC<FloatingBackgroundProps> = ({ 
  count = 15, 
  opacity = 0.05 
}) => {
  // Generate stable random values that won't change on re-renders
  const floatingElements = useMemo(() => {
    return Array.from({ length: count }).map((_, i) => ({
      key: i,
      top: Math.random() * 100,
      left: Math.random() * 100,
      animationDelay: Math.random() * 10,
      animationDuration: Math.random() * 10 + 15,
      rotation: Math.random() * 360,
      opacity: Math.random() * 0.5 + 0.5,
    }));
  }, [count]);

  return (
    <div className="absolute inset-0 -z-10 overflow-hidden" style={{ opacity }}>
      {floatingElements.map((element) => (
        <img 
          key={element.key}
          src="/neobase-logo.svg" 
          alt="" 
          className="absolute w-16 h-16 animate-float"
          style={{
            top: `${element.top}%`,
            left: `${element.left}%`,
            animationDelay: `${element.animationDelay}s`,
            animationDuration: `${element.animationDuration}s`,
            transform: `rotate(${element.rotation}deg)`,
            opacity: element.opacity,
          }}
        />
      ))}
    </div>
  );
};

export default StableFloatingBackground;