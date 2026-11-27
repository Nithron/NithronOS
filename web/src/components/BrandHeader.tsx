/**
 * The product's name in type, at the head of sign-in and setup. The drawn
 * mark waits for the identity step; it stands in the pixel display face.
 */
export default function BrandHeader({ className = '' }: { className?: string }) {
  return <p className={`pixel-display pixel-32 mb-8 text-ink ${className}`}>nOS</p>
}
