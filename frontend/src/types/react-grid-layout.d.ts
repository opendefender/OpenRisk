declare module 'react-grid-layout' {
  import React from 'react';

  export interface Layout {
    x: number;
    y: number;
    w: number;
    h: number;
    i: string;
    static?: boolean;
    minW?: number;
    minH?: number;
    maxW?: number;
    maxH?: number;
    moved?: boolean;
    placeholder?: boolean;
  }

  export interface GridLayoutProps {
    className?: string;
    layout: Layout[];
    onLayoutChange?: (layout: Layout[]) => void;
    cols?: number;
    rowHeight?: number;
    width?: number;
    isDraggable?: boolean;
    isResizable?: boolean;
    compactType?: 'vertical' | 'horizontal' | null;
    preventCollision?: boolean;
    useCSSTransforms?: boolean;
    containerPadding?: [number, number];
    margin?: [number, number];
    draggableHandle?: string;
    children?: React.ReactNode;
    onDragStart?: (layout: Layout[], oldItem: Layout, newItem: Layout, placeholder: Layout, event: MouseEvent, element: HTMLElement) => void;
    onDragStop?: (layout: Layout[], oldItem: Layout, newItem: Layout, placeholder: Layout, event: MouseEvent, element: HTMLElement) => void;
    onResizeStart?: (layout: Layout[], oldItem: Layout, newItem: Layout, placeholder: Layout, event: MouseEvent, element: HTMLElement) => void;
    onResizeStop?: (layout: Layout[], oldItem: Layout, newItem: Layout, placeholder: Layout, event: MouseEvent, element: HTMLElement) => void;
  }

  export const GridLayout: React.FC<GridLayoutProps>;

  export default GridLayout;
}

declare module 'react-resizable' {
  import React from 'react';

  export interface ResizeCallbackData {
    node: HTMLElement;
    size: { height: number; width: number };
    handle: HTMLElement;
  }

  export interface ResizableProps {
    width: number;
    height: number;
    onResize?: (event: React.SyntheticEvent, data: ResizeCallbackData) => void;
    onResizeStart?: (event: React.SyntheticEvent, data: ResizeCallbackData) => void;
    onResizeStop?: (event: React.SyntheticEvent, data: ResizeCallbackData) => void;
    draggableOpts?: object;
    children?: React.ReactNode;
    className?: string;
  }

  export const Resizable: React.FC<ResizableProps>;
}
