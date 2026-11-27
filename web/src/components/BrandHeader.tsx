/**
 * The product's name in type, at the head of sign-in and setup. The drawn
 * mark waits for the identity step; the pixel display face replaces the
 * mono stand-in when it is issued.
 */
export default function BrandHeader({ className = '' }: { className?: string }) {
  return <p className={`pixel-display mb-8 text-3xl text-ink ${className}`}>nOS</p>
}
