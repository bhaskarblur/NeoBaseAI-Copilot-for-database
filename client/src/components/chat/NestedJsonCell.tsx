import { useEffect, useRef, useState } from 'react';
import { isDateString } from '../../utils/queryUtils';

interface NestedJsonCellProps {
    data: any;
    /** Ref-map shared with the parent so expansion state survives re-renders. */
    expandedNodesRef: React.MutableRefObject<Record<string, boolean>>;
}

export default function NestedJsonCell({ data, expandedNodesRef }: NestedJsonCellProps) {
    const getFieldId = (): string => {
        let idString = '';
        if (typeof data === 'object' && data !== null) {
            if ('id' in data) {
                idString = `obj-${data.id}`;
            } else if (Array.isArray(data)) {
                idString = `arr-${data.length}-${JSON.stringify(data.slice(0, 2))
                    .split('')
                    .reduce((a, b) => (a * 31 + b.charCodeAt(0)) & 0xffffffff, 0)}`;
            } else {
                const keys = Object.keys(data).sort().join(',');
                const firstFewValues = Object.keys(data).slice(0, 2).map(key => data[key]);
                idString = `obj-${keys}-${JSON.stringify(firstFewValues)
                    .split('')
                    .reduce((a, b) => (a * 31 + b.charCodeAt(0)) & 0xffffffff, 0)}`;
            }
        }
        return `field-${idString.replace(/[^a-zA-Z0-9-]/g, '')}`;
    };

    const fieldId = getFieldId();
    const [isExpanded, setIsExpanded] = useState(() => expandedNodesRef.current[fieldId] || false);
    const expandButtonRef = useRef<HTMLDivElement>(null);
    const expandedContentRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        expandedNodesRef.current[fieldId] = isExpanded;
        if (expandButtonRef.current) {
            expandButtonRef.current.setAttribute('data-expanded', isExpanded ? 'true' : 'false');
        }
        if (expandedContentRef.current) {
            expandedContentRef.current.style.display = isExpanded ? 'block' : 'none';
        }
    }, [isExpanded, fieldId, expandedNodesRef]);

    useEffect(() => {
        const savedExpanded = expandedNodesRef.current[fieldId];
        if (savedExpanded !== undefined && savedExpanded !== isExpanded) {
            setIsExpanded(savedExpanded);
        }
        if (expandButtonRef.current) {
            const domExpanded = expandButtonRef.current.getAttribute('data-expanded') === 'true';
            if (domExpanded !== isExpanded) setIsExpanded(domExpanded);
        }
    }, [fieldId]); // eslint-disable-line react-hooks/exhaustive-deps

    const isExpandable =
        typeof data === 'object' &&
        data !== null &&
        (Array.isArray(data) ? data.length > 0 : Object.keys(data).length > 0);

    if (!isExpandable) {
        if (data === null) return <span className="text-gray-400">null</span>;
        if (data === undefined) return <span className="text-gray-400">undefined</span>;
        if (typeof data === 'boolean') return <span className="text-purple-400">{String(data)}</span>;
        if (typeof data === 'number') return <span className="text-cyan-400">{data}</span>;
        if (typeof data === 'string') {
            if (isDateString(data)) return <span className="text-yellow-300">{data}</span>;
            return <span className="text-green-400">"{data}"</span>;
        }
        return <span>{String(data)}</span>;
    }

    const handleToggleClick = () => {
        const next = !isExpanded;
        setIsExpanded(next);
        expandedNodesRef.current[fieldId] = next;
        if (expandedContentRef.current) {
            expandedContentRef.current.style.display = next ? 'block' : 'none';
        }
        if (expandButtonRef.current) {
            expandButtonRef.current.setAttribute('data-expanded', next ? 'true' : 'false');
        }
    };

    const renderExpandedContent = () => {
        if (Array.isArray(data)) {
            return (
                <div className="pl-4 mt-2 space-y-1 border-l-2 border-gray-700 pt-1">
                    {data.map((item, index) => (
                        <div key={index} className="mb-2">
                            <span className="text-gray-400 mr-1">[{index}]:</span>
                            <NestedJsonCell data={item} expandedNodesRef={expandedNodesRef} />
                        </div>
                    ))}
                </div>
            );
        }
        return (
            <div className="pl-4 mt-2 space-y-1 border-l-2 border-gray-700 pt-1">
                {Object.entries(data).map(([key, value]) => (
                    <div key={key} className="mb-2">
                        <span className="text-gray-400 mr-1">{key}:</span>
                        <NestedJsonCell data={value} expandedNodesRef={expandedNodesRef} />
                    </div>
                ))}
            </div>
        );
    };

    const getPreviewContent = () => {
        if (Array.isArray(data)) {
            const n = data.length;
            return `${n} item${n !== 1 ? 's' : ''} in list`;
        }
        const keys = Object.keys(data);
        if ('id' in data && ('name' in data || 'title' in data)) {
            const nameField = data.name || data.title;
            return typeof nameField === 'string'
                ? `${nameField} (${keys.length} properties)`
                : `Details with ${keys.length} properties`;
        }
        const previewKeys = keys.slice(0, 2);
        return previewKeys.length > 0
            ? `View: ${previewKeys.join(', ')}${keys.length > 2 ? '...' : ''}`
            : `${keys.length} propert${keys.length !== 1 ? 'ies' : 'y'}`;
    };

    return (
        <div
            className={`nested-json min-w-[160px] ${isExpanded ? 'mt-2' : ''}`}
            style={{ position: 'relative', zIndex: 5 }}
            data-field-id={fieldId}
        >
            <div
                ref={expandButtonRef}
                className="cursor-pointer flex items-center transition-colors"
                tabIndex={0}
                role="button"
                aria-expanded={isExpanded}
                data-expanded={isExpanded ? 'true' : 'false'}
                onClick={e => { e.preventDefault(); e.stopPropagation(); handleToggleClick(); }}
                onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleToggleClick(); } }}
            >
                <span className="mr-2 text-white font-medium">
                    {isExpanded ? (
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="inline-block">
                            <path d="m18 15-6-6-6 6" />
                        </svg>
                    ) : (
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="inline-block">
                            <path d="m6 9 6 6 6-6" />
                        </svg>
                    )}
                </span>
                <span className="text-blue-400 font-medium">{getPreviewContent()}</span>
            </div>
            <div
                ref={expandedContentRef}
                style={{ display: isExpanded ? 'block' : 'none' }}
                data-expanded-content={fieldId}
            >
                {renderExpandedContent()}
            </div>
        </div>
    );
}
