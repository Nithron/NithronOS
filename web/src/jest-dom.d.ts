/// <reference types="@testing-library/jest-dom" />

/* eslint-disable @typescript-eslint/no-empty-object-type -- declaration merging needs empty interfaces */
declare namespace Vi {
  interface Assertion<T = any> extends jest.Matchers<void, T> {}
  interface AsymmetricMatchersContaining extends jest.AsymmetricMatchers {}
}
