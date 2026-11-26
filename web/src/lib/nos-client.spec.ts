import { describe, it, expect, vi, beforeEach } from 'vitest';

// Hoist axios mock before importing the module under test
vi.mock('axios', () => {
	const instance = {
		get: vi.fn(),
		post: vi.fn(),
		put: vi.fn(),
		patch: vi.fn(),
		delete: vi.fn(),
		interceptors: {
			request: { use: vi.fn() },
			response: { use: vi.fn() },
		},
	};
	const create = vi.fn(() => instance);
	return {
		default: { create },
		create,
	};
});

import axios from 'axios';
import http, { openSSE, openWS, ProxyMisconfiguredError } from './nos-client';


describe('nos-client', () => {
	it('rejects an HTML answer as a misconfigured proxy', async () => {
		const instance = (axios.create as any).mock.results[0].value;
		const onSuccess = instance.interceptors.response.use.mock.calls[0][0];
		await expect(onSuccess({ status: 200, headers: { 'content-type': 'text/html; charset=utf-8' }, data: '<!doctype html>' }))
			.rejects.toBeInstanceOf(ProxyMisconfiguredError);
		const ok = { status: 200, headers: { 'content-type': 'application/json' }, data: {} };
		expect(onSuccess(ok)).toBe(ok);
	});

	let mockAxiosInstance: any;

	beforeEach(() => {
		mockAxiosInstance = (axios.create as any).mock.results[0].value;
		mockAxiosInstance.get.mockReset();
		mockAxiosInstance.post.mockReset();
		mockAxiosInstance.put.mockReset();
		mockAxiosInstance.patch.mockReset();
		mockAxiosInstance.delete.mockReset();
	});

	describe('Path validation', () => {
		it('should correctly build URLs with /api prefix', async () => {
			mockAxiosInstance.get.mockResolvedValue({ data: { test: 'data' } });
			await http.get('/v1/ping');
			expect(mockAxiosInstance.get).toHaveBeenCalledWith('/v1/ping', { params: undefined });
		});

		it('should throw error if path includes /api in development', async () => {
			const originalEnv = process.env.NODE_ENV;
			process.env.NODE_ENV = 'development';
			await expect(http.get('/api/v1/ping')).rejects.toThrow("Do not include '/api' in paths");
			process.env.NODE_ENV = originalEnv;
		});

		it('should allow /setup/ paths', async () => {
			mockAxiosInstance.get.mockResolvedValue({ data: { firstBoot: true } });
			await http.get('/v1/setup/state');
			expect(mockAxiosInstance.get).toHaveBeenCalledWith('/v1/setup/state', { params: undefined });
		});

		it('should allow various unversioned paths', async () => {
			const paths = [
				'/auth/login',
				'/metrics',
				'/pools',
				'/disks',
				'/shares',
				'/apps',
				'/snapshots/recent',
				'/support/bundle',
			];

			for (const path of paths) {
				mockAxiosInstance.get.mockResolvedValue({ data: {} });
				await http.get(path);
				expect(mockAxiosInstance.get).toHaveBeenCalledWith(path, expect.any(Object));
			}
		});
	});

	describe('SSE helper', () => {
		it('should create EventSource with correct URL', () => {
			const originalLocation = window.location;
			delete (window as any).location;
			(window as any).location = { origin: 'http://localhost:3000' } as any;
			const mockEventSource = vi.fn();
			(global as any).EventSource = mockEventSource;
			openSSE('/v1/updates/progress/stream');
			expect(mockEventSource).toHaveBeenCalledWith(
				'http://localhost:3000/api/v1/updates/progress/stream',
				{ withCredentials: true }
			);
			(window as any).location = originalLocation;
		});
	});

	describe('WebSocket helper', () => {
		it('should create WebSocket with correct URL for HTTP', () => {
			const originalLocation = window.location;
			delete (window as any).location;
			(window as any).location = { protocol: 'http:', host: 'localhost:3000' } as any;
			const mockWebSocket = vi.fn();
			(global as any).WebSocket = mockWebSocket;
			openWS('/v1/apps/123/logs');
			expect(mockWebSocket).toHaveBeenCalledWith('ws://localhost:3000/api/v1/apps/123/logs');
			(window as any).location = originalLocation;
		});

		it('should create WebSocket with correct URL for HTTPS', () => {
			const originalLocation = window.location;
			delete (window as any).location;
			(window as any).location = { protocol: 'https:', host: 'app.example.com' } as any;
			const mockWebSocket = vi.fn();
			(global as any).WebSocket = mockWebSocket;
			openWS('/v1/apps/123/logs');
			expect(mockWebSocket).toHaveBeenCalledWith('wss://app.example.com/api/v1/apps/123/logs');
			(window as any).location = originalLocation;
		});
	});

	describe('Endpoint helpers', () => {
		it('should have all required endpoint groups', () => {
			const requiredGroups = [
				'setup',
				'auth',
				'system',
				'network',
				'telemetry',
				'health',
				'storage',
				'pools',
				'disks',
				'snapshots',
				'updates',
				'shares',
				'smb',
				'apps',
				'support',
				'monitoring',
				'scrub',
				'balance',
				'schedules',
				'jobs',
				'devices',
				'smart',
			];
			for (const group of requiredGroups) {
				expect(http).toHaveProperty(group);
				expect(typeof (http as any)[group]).toBe('object');
			}
		});

		it('should call correct endpoints for auth operations', async () => {
			mockAxiosInstance.post.mockResolvedValue({ data: { token: 'test' } });
			await http.auth.login({ username: 'admin', password: 'pass' });
			expect(mockAxiosInstance.post).toHaveBeenCalledWith('/v1/auth/login', { username: 'admin', password: 'pass' });
		});

		it('should handle blob downloads for support bundle', async () => {
			const mockBlob = new Blob(['test']);
			mockAxiosInstance.get.mockResolvedValue({ data: mockBlob });
			const result = await http.support.bundle();
			expect(mockAxiosInstance.get).toHaveBeenCalledWith('/support/bundle', { params: undefined, responseType: 'blob' });
			expect(result).toBe(mockBlob);
		});
	});

	describe('CSRF handling', () => {
		it('should add CSRF header if cookie exists (interceptor registered)', () => {
			// Ensure request interceptor registration happened
			expect((axios.create as any).mock.results[0].value.interceptors.request.use).toBeCalled();
		});
	});
});
