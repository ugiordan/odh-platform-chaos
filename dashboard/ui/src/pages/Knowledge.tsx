import { useState, useEffect } from 'react';
import { Spinner, Alert } from '@patternfly/react-core';
import { useApi } from '../api/hooks';
import { apiUrl } from '../api/client';
import type { ComponentModel, ManagedResource, ListResult, Experiment } from '../types/api';
import './Knowledge.css';

const KIND_COLORS: Record<string, string> = {
  Deployment: '#06c',
  ConfigMap: '#3e8635',
  ServiceAccount: '#8a8d90',
  ClusterRole: '#6753ac',
  ClusterRoleBinding: '#6753ac',
  Service: '#f0ab00',
  ValidatingWebhookConfig: '#c9190b',
  MutatingWebhookConfig: '#c9190b',
  CRD: '#004080',
};

function getKindColor(kind: string): string {
  return KIND_COLORS[kind] || '#8a8d90';
}

interface CoverageData {
  color: string;
  count: number;
}

function computeCoverage(
  resource: ManagedResource,
  experiments: Experiment[],
  component: string
): CoverageData {
  let matchedExperiments: Experiment[] = [];

  // P3-9: Match by resource kind to experiment type AND filter by component
  if (resource.kind === 'Deployment') {
    matchedExperiments = experiments.filter(e => e.type === 'PodKill' && e.component === component);
  } else if (resource.kind === 'ConfigMap') {
    matchedExperiments = experiments.filter(e => e.type === 'ConfigDrift' && e.component === component);
  } else if (resource.kind === 'ValidatingWebhookConfig' || resource.kind === 'MutatingWebhookConfig') {
    matchedExperiments = experiments.filter(e => e.type === 'WebhookDisrupt' && e.component === component);
  } else if (resource.kind === 'ClusterRole' || resource.kind === 'ClusterRoleBinding') {
    matchedExperiments = experiments.filter(e => e.type === 'RBACRevoke' && e.component === component);
  } else if (resource.kind === 'CRD') {
    matchedExperiments = experiments.filter(e => e.type === 'CRDMutation' && e.component === component);
  } else {
    // For other kinds, check if any experiments exist for this component
    matchedExperiments = experiments.filter(e => e.component === component);
  }

  if (matchedExperiments.length === 0) {
    return { color: '#d2d2d2', count: 0 }; // gray - not tested
  }

  const hasResilient = matchedExperiments.some(e => e.verdict === 'Resilient');
  const hasDegraded = matchedExperiments.some(e => e.verdict === 'Degraded');
  const hasFailed = matchedExperiments.some(e => e.verdict === 'Failed');

  if (hasFailed) {
    return { color: '#c9190b', count: matchedExperiments.length }; // red
  } else if (hasDegraded) {
    return { color: '#f0ab00', count: matchedExperiments.length }; // yellow
  } else if (hasResilient) {
    return { color: '#3e8635', count: matchedExperiments.length }; // green
  }

  return { color: '#d2d2d2', count: matchedExperiments.length }; // gray - inconclusive
}

interface DependencyGraphProps {
  component: ComponentModel;
  experiments: Experiment[];
  zoom: number;
}

function DependencyGraph({ component, experiments, zoom }: DependencyGraphProps) {
  const resources = component.managedResources;
  const centerX = 350;
  const startY = 40;
  const nodeWidth = 180;
  const nodeHeight = 48;
  const rowGap = 80;
  const colGap = 20;
  const nodesPerRow = 3;

  // Controller node
  const controllerNode = {
    x: centerX - nodeWidth / 2,
    y: startY,
    width: nodeWidth,
    height: nodeHeight,
    label: 'Controller',
    name: component.controller,
  };

  // Layout managed resources in rows
  const resourceNodes = resources.map((resource, idx) => {
    const row = Math.floor(idx / nodesPerRow);
    const col = idx % nodesPerRow;
    const totalInRow = Math.min(nodesPerRow, resources.length - row * nodesPerRow);
    const rowWidth = totalInRow * nodeWidth + (totalInRow - 1) * colGap;
    const rowStartX = centerX - rowWidth / 2;

    const coverage = computeCoverage(resource, experiments, component.name);

    return {
      x: rowStartX + col * (nodeWidth + colGap),
      y: startY + nodeHeight + rowGap + row * (nodeHeight + rowGap),
      width: nodeWidth,
      height: nodeHeight,
      kind: resource.kind,
      name: resource.name,
      color: coverage.color,
      count: coverage.count,
    };
  });

  const viewBoxWidth = 700 / zoom;
  const viewBoxHeight = 500 / zoom;
  const viewBoxX = (700 - viewBoxWidth) / 2;
  const viewBoxY = (500 - viewBoxHeight) / 2;

  return (
    <svg viewBox={`${viewBoxX} ${viewBoxY} ${viewBoxWidth} ${viewBoxHeight}`}>
      {/* Lines from controller to resources */}
      {resourceNodes.map((node, idx) => (
        <line
          key={idx}
          x1={controllerNode.x + controllerNode.width / 2}
          y1={controllerNode.y + controllerNode.height}
          x2={node.x + node.width / 2}
          y2={node.y}
          stroke="#d2d2d2"
          strokeWidth="2"
        />
      ))}

      {/* Controller node */}
      <g className="node-box">
        <rect
          x={controllerNode.x}
          y={controllerNode.y}
          width={controllerNode.width}
          height={controllerNode.height}
          fill="#fafafa"
          stroke="#06c"
        />
        <text
          x={controllerNode.x + controllerNode.width / 2}
          y={controllerNode.y + 20}
          textAnchor="middle"
          className="node-label"
        >
          {controllerNode.label}
        </text>
        <text
          x={controllerNode.x + controllerNode.width / 2}
          y={controllerNode.y + 34}
          textAnchor="middle"
          className="node-type"
        >
          {controllerNode.name}
        </text>
      </g>

      {/* Resource nodes */}
      {resourceNodes.map((node, idx) => (
        <g key={idx} className="node-box">
          <rect
            x={node.x}
            y={node.y}
            width={node.width}
            height={node.height}
            fill={node.color}
            fillOpacity="0.15"
            stroke={node.color}
          />
          <text
            x={node.x + node.width / 2}
            y={node.y + 20}
            textAnchor="middle"
            className="node-label"
          >
            {node.kind}
          </text>
          <text
            x={node.x + node.width / 2}
            y={node.y + 34}
            textAnchor="middle"
            className="node-type"
          >
            {node.name}
          </text>
          {node.count > 0 && (
            <g>
              <circle
                cx={node.x + node.width - 12}
                cy={node.y + 12}
                r="10"
                fill={node.color}
              />
              <text
                x={node.x + node.width - 12}
                y={node.y + 16}
                textAnchor="middle"
                fill="white"
                fontSize="10"
                fontWeight="700"
              >
                {node.count}
              </text>
            </g>
          )}
        </g>
      ))}
    </svg>
  );
}

interface DetailPanelProps {
  component: ComponentModel;
  operator: string;
  experiments: Experiment[];
}

function DetailPanel({ component, operator, experiments }: DetailPanelProps) {
  const namespace = experiments.length > 0 && experiments[0] ? experiments[0].namespace : '—';

  const testedCount = component.managedResources.filter(r => {
    const coverage = computeCoverage(r, experiments, component.name);
    return coverage.count > 0;
  }).length;

  const untestedCount = component.managedResources.length - testedCount;

  return (
    <div className="detail-panel">
      <div className="panel-header">Component Details</div>

      <div className="panel-section">
        <h4>Component Info</h4>
        <div style={{ fontSize: 13, lineHeight: '1.6' }}>
          <div><strong>Operator:</strong> {operator}</div>
          <div><strong>Component:</strong> {component.name}</div>
          <div><strong>Namespace:</strong> {namespace}</div>
        </div>
      </div>

      <div className="panel-section">
        <h4>Managed Resources</h4>
        {component.managedResources.map((resource, idx) => {
          const coverage = computeCoverage(resource, experiments, component.name);
          let coverageTag = 'uncovered';
          let coverageLabel = 'Untested';

          if (coverage.count > 0) {
            if (coverage.color === '#3e8635') {
              coverageTag = 'covered';
              coverageLabel = 'Resilient';
            } else if (coverage.color === '#f0ab00') {
              coverageTag = 'partial';
              coverageLabel = 'Degraded';
            } else if (coverage.color === '#c9190b') {
              coverageTag = 'partial';
              coverageLabel = 'Failed';
            } else {
              coverageTag = 'partial';
              coverageLabel = 'Tested';
            }
          }

          return (
            <div key={idx} className="resource-row">
              <div
                className="resource-icon"
                style={{ backgroundColor: getKindColor(resource.kind) }}
              >
                {resource.kind.substring(0, 2).toUpperCase()}
              </div>
              <div style={{ flex: 1 }}>
                <div className="resource-name">{resource.name}</div>
                {resource.namespace && <div className="resource-ns">{resource.namespace}</div>}
              </div>
              <span className={`coverage-tag ${coverageTag}`}>{coverageLabel}</span>
            </div>
          );
        })}
      </div>

      <div className="panel-section">
        <h4>Chaos Coverage Summary</h4>
        <div className="coverage-summary">
          <div className="coverage-summary-item" style={{ background: '#e6f9e6' }}>
            <strong>{testedCount}</strong> Tested
          </div>
          <div className="coverage-summary-item" style={{ background: '#fce8e6' }}>
            <strong>{untestedCount}</strong> Untested
          </div>
        </div>
      </div>
    </div>
  );
}

export function Knowledge() {
  const [selectedOperator, setSelectedOperator] = useState<string>('');
  const [selectedComponent, setSelectedComponent] = useState<string>('');
  const [zoom, setZoom] = useState<number>(1);

  // Fetch operators
  const { data: operators, loading: loadingOps, error: errorOps } = useApi<string[]>(
    apiUrl('/operators')
  );

  // Fetch components for selected operator
  const { data: components, loading: loadingComps } = useApi<string[]>(
    selectedOperator ? apiUrl(`/operators/${selectedOperator}/components`) : null
  );

  // Fetch knowledge for selected component
  const { data: knowledge, loading: loadingKnowledge } = useApi<ComponentModel>(
    selectedOperator && selectedComponent
      ? apiUrl(`/knowledge/${selectedOperator}/${selectedComponent}`)
      : null
  );

  // Fetch experiments for coverage overlay
  const { data: experimentsData } = useApi<ListResult>(
    selectedOperator && selectedComponent
      ? apiUrl('/experiments', { operator: selectedOperator, component: selectedComponent, pageSize: '200' })
      : null
  );

  const experiments = experimentsData?.items || [];
  const showTruncationWarning = experimentsData && experimentsData.totalCount > experimentsData.items.length;

  // Auto-select first operator
  useEffect(() => {
    if (operators && operators.length > 0 && !selectedOperator) {
      const firstOp = operators[0];
      if (firstOp) {
        setSelectedOperator(firstOp);
      }
    }
  }, [operators, selectedOperator]);

  // Auto-select first component when operator changes
  useEffect(() => {
    if (components && components.length > 0) {
      const firstComp = components[0];
      if (firstComp) {
        setSelectedComponent(firstComp);
      }
    } else {
      setSelectedComponent('');
    }
  }, [components]);

  const handleZoomIn = () => setZoom(Math.min(zoom + 0.2, 2));
  const handleZoomOut = () => setZoom(Math.max(zoom - 0.2, 0.5));
  const handleZoomReset = () => setZoom(1);

  if (loadingOps) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
        <Spinner aria-label="Loading" />
      </div>
    );
  }

  if (errorOps) {
    return (
      <div style={{ padding: 24 }}>
        <Alert variant="danger" title="Failed to load operators">{errorOps}</Alert>
      </div>
    );
  }

  return (
    <>
      <div className="knowledge-toolbar">
        <span className="toolbar-label">Operator:</span>
        <select
          className="toolbar-select"
          value={selectedOperator}
          onChange={(e) => setSelectedOperator(e.target.value)}
        >
          <option value="">Select operator...</option>
          {operators?.map((op) => (
            <option key={op} value={op}>{op}</option>
          ))}
        </select>

        <span className="toolbar-label">Component:</span>
        <select
          className="toolbar-select"
          value={selectedComponent}
          onChange={(e) => setSelectedComponent(e.target.value)}
          disabled={!selectedOperator || loadingComps}
        >
          <option value="">Select component...</option>
          {components?.map((comp) => (
            <option key={comp} value={comp}>{comp}</option>
          ))}
        </select>
      </div>

      {!selectedComponent ? (
        <div style={{ padding: 80, textAlign: 'center', color: '#6a6e73' }}>
          Select an operator and component to view the dependency graph.
        </div>
      ) : loadingKnowledge ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
          <Spinner aria-label="Loading" />
        </div>
      ) : !knowledge ? (
        <div style={{ padding: 24 }}>
          <Alert variant="info" title="No data available" isInline>
            No knowledge data found for this component.
          </Alert>
        </div>
      ) : (
        <div className="knowledge-content">
          <div className="graph-card">
            <div className="graph-header">
              <span>Dependency Graph — {knowledge.name}</span>
              <div className="graph-controls">
                <button className="graph-btn" onClick={handleZoomOut}>−</button>
                <button className="graph-btn" onClick={handleZoomReset}>Reset</button>
                <button className="graph-btn" onClick={handleZoomIn}>+</button>
              </div>
            </div>
            {showTruncationWarning && (
              <div style={{ padding: '8px 16px', fontSize: 13, color: '#6a6e73', background: '#f0f0f0', borderBottom: '1px solid #d2d2d2' }}>
                Showing {experimentsData.items.length} of {experimentsData.totalCount} experiments
              </div>
            )}
            <div className="graph-area">
              <DependencyGraph component={knowledge} experiments={experiments} zoom={zoom} />
            </div>
            <div className="graph-legend">
              <div className="legend-item">
                <div className="legend-color" style={{ background: '#3e8635' }} />
                <span>Resilient</span>
              </div>
              <div className="legend-item">
                <div className="legend-color" style={{ background: '#f0ab00' }} />
                <span>Degraded</span>
              </div>
              <div className="legend-item">
                <div className="legend-color" style={{ background: '#c9190b' }} />
                <span>Failed</span>
              </div>
              <div className="legend-item">
                <div className="legend-color" style={{ background: '#d2d2d2' }} />
                <span>Not tested</span>
              </div>
            </div>
          </div>

          <DetailPanel component={knowledge} operator={selectedOperator} experiments={experiments} />
        </div>
      )}
    </>
  );
}
