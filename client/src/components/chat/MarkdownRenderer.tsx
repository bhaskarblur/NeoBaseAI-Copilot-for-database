import React from 'react';
import ReactMarkdown from 'react-markdown';
import 'highlight.js/styles/github.css';
import './MarkdownRenderer.css';
import { highlightSearchText } from '../../utils/highlightSearch';

interface MarkdownRendererProps {
  markdown: string;
  className?: string;
  searchQuery?: string;
}

const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ markdown, className = '', searchQuery }) => {
  // Apply a safeguard to ensure markdown is always a string
  const safeMarkdown = typeof markdown === 'string' ? markdown : '';
  
  return (
    <div className={`markdown-renderer ${className}`}>
      <ReactMarkdown
        skipHtml={false}
        components={{
          // Customize other components as needed
          a({ node, children, ...props }: any) {
            return (
              <a 
                {...props} 
                target="_blank" 
                rel="noopener noreferrer"
              >
                {children}
              </a>
            );
          },
          table({ node, children, ...props }: any) {
            return (
              <div className="overflow-x-auto">
                <table className="neo-border" {...props}>
                  {children}
                </table>
              </div>
            );
          },
          img({ node, ...props }: any) {
            return (
              <img 
                {...props} 
                className="neo-border my-2" 
                alt={props.alt || 'Image'} 
              />
            );
          },
          blockquote({ node, children, ...props }: any) {
            return (
              <blockquote className="neo-border" {...props}>
                {children}
              </blockquote>
            );
          },
          pre({ node, children, ...props }: any) {
            return (
              <pre className="neo-border" {...props}>
                {children}
              </pre>
            );
          },
          ul({ node, children, ...props }: any) {
            return (
              <ul className="list-disc ml-6" {...props}>
                {children}
              </ul>
            );
          },
          ol({ node, children, ...props }: any) {
            return (
              <ol className="list-decimal ml-6" {...props}>
                {children}
              </ol>
            );
          },
          p({ node, children, ...props }: any) {
            // Recursive function to process children
            const processChildren = (child: any): any => {
              if (typeof child === 'string' && searchQuery) {
                return highlightSearchText(child, searchQuery);
              }
              if (Array.isArray(child)) {
                return child.map((c, i) => <React.Fragment key={i}>{processChildren(c)}</React.Fragment>);
              }
              if (React.isValidElement(child) && child.props.children) {
                return React.cloneElement(child, {
                  ...child.props,
                  children: processChildren(child.props.children)
                });
              }
              return child;
            };
            
            return <p {...props}>{processChildren(children)}</p>;
          },
          // Also handle text nodes directly
          text({ node, ...props }: any) {
            if (searchQuery && props.children) {
              return <>{highlightSearchText(props.children, searchQuery)}</>;
            }
            return <>{props.children}</>;
          },
          // Handle list items
          li({ node, children, ...props }: any) {
            const processChildren = (child: any): any => {
              if (typeof child === 'string' && searchQuery) {
                return highlightSearchText(child, searchQuery);
              }
              if (Array.isArray(child)) {
                return child.map((c, i) => <React.Fragment key={i}>{processChildren(c)}</React.Fragment>);
              }
              if (React.isValidElement(child) && child.props.children) {
                return React.cloneElement(child, {
                  ...child.props,
                  children: processChildren(child.props.children)
                });
              }
              return child;
            };
            
            return <li {...props}>{processChildren(children)}</li>;
          },
          // Handle inline code
          code({ node, inline, children, ...props }: any) {
            const match = /language-(\w+)/.exec(props.className || '');
            const isInline = inline || !match;
            
            if (isInline && searchQuery && typeof children === 'string') {
              return (
                <code className={props.className} {...props}>
                  {highlightSearchText(children, searchQuery)}
                </code>
              );
            }
            
            return !isInline ? (
              <div className="code-block-wrapper neo-border">
                <div className="code-language-indicator">{match ? match[1] : 'code'}</div>
                <code
                  className={props.className}
                  {...props}
                >
                  {children}
                </code>
              </div>
            ) : (
              <code className={props.className} {...props}>
                {children}
              </code>
            );
          }
        }}
      >
        {safeMarkdown}
      </ReactMarkdown>

    </div>
  );
};

export default MarkdownRenderer; 