interface LayerMenuProps {
  x: number;
  y: number;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onBringToFront: () => void;
  onSendToBack: () => void;
}

export function LayerMenu({ x, y, onMoveUp, onMoveDown, onBringToFront, onSendToBack }: LayerMenuProps) {
  return (
    <div className="layer-menu" style={{ left: x, top: y }} role="menu" aria-label="图层顺序">
      <button type="button" role="menuitem" onClick={onMoveUp}>上移一层</button>
      <button type="button" role="menuitem" onClick={onMoveDown}>下移一层</button>
      <button type="button" role="menuitem" onClick={onBringToFront}>置于顶层</button>
      <button type="button" role="menuitem" onClick={onSendToBack}>置于底层</button>
    </div>
  );
}

