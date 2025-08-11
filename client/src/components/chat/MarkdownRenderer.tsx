import React from 'react';
import ReactMarkdown from 'react-markdown';
import type { Components } from 'react-markdown';
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

  // Recursive function to process children for search highlighting
  const processChildren = (child: React.ReactNode): React.ReactNode => {
    if (typeof child === 'string' && searchQuery) {
      return highlightSearchText(child, searchQuery);
    }
    if (Array.isArray(child)) {
      return child.map((c, i) => <React.Fragment key={i}>{processChildren(c)}</React.Fragment>);
    }
    if (React.isValidElement(child) && child.props && 'children' in child.props) {
      return React.cloneElement(child, {
        ...(child.props as any),
        children: processChildren(child.props.children)
      } as any);
    }
    return child;
  };

  // Define the components with proper typing
  const components: Partial<Components> = {
    a: (props: any) => {
      const { children, ...rest } = props;
      return (
        <a 
          {...rest} 
          target="_blank" 
          rel="noopener noreferrer"
        >
          {children}
        </a>
      );
    },
    table: (props: any) => {
      const { children, ...rest } = props;
      return (
        <div className="overflow-x-auto">
          <table className="neo-border" {...rest}>
            {children}
          </table>
        </div>
      );
    },
    img: (props: any) => {
      return (
        <img 
          {...props} 
          className="neo-border my-2" 
          alt={props.alt || 'Image'} 
        />
      );
    },
    blockquote: (props: any) => {
      const { children, ...rest } = props;
      return (
        <blockquote className="neo-border" {...rest}>
          {children}
        </blockquote>
      );
    },
    pre: (props: any) => {
      const { children, ...rest } = props;
      return (
        <pre className="neo-border" {...rest}>
          {children}
        </pre>
      );
    },
    ul: (props: any) => {
      const { children, ...rest } = props;
      return (
        <ul className="list-disc ml-6" {...rest}>
          {children}
        </ul>
      );
    },
    ol: (props: any) => {
      const { children, ...rest } = props;
      return (
        <ol className="list-decimal ml-6" {...rest}>
          {children}
        </ol>
      );
    },
    p: (props: any) => {
      const { children, ...rest } = props;
      return <p {...rest}>{processChildren(children)}</p>;
    },
    // Also handle text nodes directly
    text: (props: any) => {
      if (searchQuery && props.children) {
        return <>{highlightSearchText(props.children as string, searchQuery)}</>;
      }
      return <>{props.children}</>;
    },
    // Handle list items
    li: (props: any) => {
      const { children, ...rest } = props;
      return <li className="block" {...rest}>{processChildren(children)}</li>;
    },
    // Handle inline code
    code: (props: any) => {
      const { inline, children, className, ...rest } = props;
      const match = /language-(\w+)/.exec(className || '');
      const isInline = inline || !match;
      
      if (isInline && searchQuery && typeof children === 'string') {
        return (
          <code className={className} {...rest}>
            {highlightSearchText(children, searchQuery)}
          </code>
        );
      }
      
      return !isInline ? (
        <div className="code-block-wrapper neo-border">
          <div className="code-language-indicator">{match ? match[1] : 'code'}</div>
          <code
            className={className}
            {...rest}
          >
            {children}
          </code>
        </div>
      ) : (
        <code className={className} {...rest}>
          {children}
        </code>
      );
    }
  };
  
  return (
    <div className={`markdown-renderer ${className}`}>
      <ReactMarkdown
        skipHtml={false}
        components={components}
      >
        {safeMarkdown}
      </ReactMarkdown>
    </div>
  );
};

export default MarkdownRenderer;