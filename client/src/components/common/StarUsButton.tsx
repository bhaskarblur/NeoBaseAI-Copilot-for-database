import { Github } from 'lucide-react';
import { useEffect, useState } from 'react';

interface StarUsButtonProps {
    className?: string;
    isMobile?: boolean;
}

export default function StarUsButton({ className = '', isMobile = false }: StarUsButtonProps) {
    const [starCount, setStarCount] = useState<number | null>(null);

    useEffect(() => {
        const fetchStarCount = async () => {
            try {
                const response = await fetch(`${import.meta.env.VITE_API_URL}/github/stats`);
                if (!response.ok) {
                    throw new Error('Failed to fetch star count');
                }
                const data = await response.json();
                setStarCount(data.data.star_count);
            } catch (error) {
                console.error('Error fetching star count:', error);
                setStarCount(1); // I Starred it manually :)
            }
        };

        fetchStarCount();
    }, []);

    const formatStarCount = (count: number): string => {
        if (count >= 1000) {
            return `${(count / 1000).toFixed(1)}k`;
        }
        return count.toString();
    };

    return (
        <a
            href="https://github.com/bhaskarblur/neobase-ai-dba"
            target="_blank"
            rel="noopener noreferrer"
            className={`
                ${isMobile ? 'flex' : 'hidden md:flex'}
                ${isMobile ? 'relative' : 'fixed bottom-4 right-4'}
                z-[999] 
                neo-button 
                items-center 
                gap-2 
                ${isMobile ? 'px-3 py-1.5' : 'px-4 py-2'}
                text-sm 
                font-bold
                hover:translate-y-[-2px]
                hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)]
                active:translate-y-[0px]
                active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]
                transition-all
                duration-200
                ${className}
            `}
        >
            <Github className="w-4 h-4" />
            {isMobile ? (
                <span className="
                    bg-white/20 
                    px-2 
                    py-0.5 
                    rounded-full 
                    text-xs 
                    font-mono
                ">{formatStarCount(starCount || 60)}</span>
            ) : (
                <>
                    <span>Star Us</span>
                    <span className="
                        bg-white/20 
                        px-2 
                        py-0.5 
                        rounded-full 
                        text-xs 
                        font-mono
                    ">{formatStarCount(starCount || 60)}</span>
                </>
            )}
        </a>
    );
} 