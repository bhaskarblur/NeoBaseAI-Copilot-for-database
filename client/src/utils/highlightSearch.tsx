import React from 'react';

export const highlightSearchText = (text: string, searchQuery: string): React.ReactNode => {
  if (!searchQuery || !text) {
    return text;
  }

  try {
    // Escape special regex characters
    const escapedQuery = searchQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(`(${escapedQuery})`, 'gi');
    const parts = text.split(regex);
    
    return (
      <>
        {parts.map((part, index) => {
          // Check if this part matches the search query (case-insensitive)
          if (part.toLowerCase() === searchQuery.toLowerCase()) {
            return (
              <mark key={index} style={{ backgroundColor: '#FFEB3B', color: '#000', padding: '0 2px', borderRadius: '2px', fontWeight: 500 }}>
                {part}
              </mark>
            );
          }
          return part;
        })}
      </>
    );
  } catch (e) {
    // If regex fails, return original text
    return text;
  }
};

export const getHighlightedContent = (content: string, searchQuery: string): React.ReactNode => {
  if (!searchQuery) return content;
  
  // Split by newlines to preserve formatting
  const lines = content.split('\n');
  
  return lines.map((line, lineIndex) => (
    <React.Fragment key={lineIndex}>
      {lineIndex > 0 && '\n'}
      {highlightSearchText(line, searchQuery)}
    </React.Fragment>
  ));
};