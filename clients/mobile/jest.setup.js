/* global jest */
// Native modules the JavaScript under test reaches. Each test file can
// replace these with its own mocks.

jest.mock('react-native-keychain', () => {
  let entry = false;
  return {
    ACCESSIBLE: {
      AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY:
        'AccessibleAfterFirstUnlockThisDeviceOnly',
    },
    getGenericPassword: jest.fn(async () => entry),
    setGenericPassword: jest.fn(async (username, password, options) => {
      entry = {username, password, service: options.service, storage: 'test'};
      return {service: options.service, storage: 'test'};
    }),
    resetGenericPassword: jest.fn(async () => {
      entry = false;
      return true;
    }),
  };
});

jest.mock('react-native-vision-camera', () => ({
  Camera: 'Camera',
  useCameraDevice: jest.fn(() => undefined),
  useCameraPermission: jest.fn(() => ({
    hasPermission: false,
    requestPermission: jest.fn(async () => false),
  })),
  useCodeScanner: jest.fn(options => options),
}));
