/**
 * Authentication module for Leads Core Admin Panel
 */

class AuthManager {
    constructor() {
        this.tokenKey = 'leads-core-token';
        this.userKey = 'leads-core-user';
        this.demoModeKey = 'leads-core-demo-mode';
    }

    /**
     * Check if user is authenticated
     */
    isAuthenticated() {
        // Check if demo mode is enabled
        if (this.isDemoMode()) {
            return true;
        }

        const token = this.getToken();
        if (!token) return false;

        try {
            const payload = this.decodeJWT(token);
            return payload.exp > Date.now() / 1000;
        } catch {
            return false;
        }
    }

    /**
     * Check if in demo mode
     */
    isDemoMode() {
        return localStorage.getItem(this.demoModeKey) === 'true';
    }

    /**
     * Enable demo mode
     */
    enableDemoMode() {
        localStorage.setItem(this.demoModeKey, 'true');
        localStorage.setItem(this.userKey, JSON.stringify({
            id: 'demo',
            username: 'demo',
            plan: 'demo',
            loginTime: new Date().toISOString()
        }));
    }

    /**
     * Disable demo mode
     */
    disableDemoMode() {
        localStorage.removeItem(this.demoModeKey);
    }

    /**
     * Get stored token
     */
    getToken() {
        return localStorage.getItem(this.tokenKey);
    }

    /**
     * Get stored user info
     */
    getUser() {
        const user = localStorage.getItem(this.userKey);
        return user ? JSON.parse(user) : null;
    }

    /**
     * Generate JWT token
     */
    async generateToken(secret, userId) {
        try {
            const header = {
                alg: 'HS256',
                typ: 'JWT'
            };

            const payload = {
                user_id: userId,
                iat: Math.floor(Date.now() / 1000),
                exp: Math.floor(Date.now() / 1000) + (24 * 60 * 60) // 24 hours
            };

            const encodedHeader = this.base64UrlEncode(JSON.stringify(header));
            const encodedPayload = this.base64UrlEncode(JSON.stringify(payload));
            const data = `${encodedHeader}.${encodedPayload}`;

            const key = await crypto.subtle.importKey(
                'raw',
                new TextEncoder().encode(secret),
                { name: 'HMAC', hash: 'SHA-256' },
                false,
                ['sign']
            );

            const signature = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(data));
            const encodedSignature = this.base64UrlEncode(new Uint8Array(signature));

            return `${data}.${encodedSignature}`;
        } catch (error) {
            console.error('Error generating JWT:', error);
            throw new Error('Failed to generate JWT token');
        }
    }

    /**
     * Login with credentials
     */
    async login(secret, userId) {
        try {
            const token = await this.generateToken(secret, userId);
            
            // Store token and user info
            localStorage.setItem(this.tokenKey, token);
            localStorage.setItem(this.userKey, JSON.stringify({ 
                id: userId, 
                loginTime: new Date().toISOString() 
            }));

            return { success: true, token };
        } catch (error) {
            console.error('Login error:', error);
            return { success: false, error: error.message };
        }
    }

    /**
     * Logout user
     */
    logout() {
        localStorage.removeItem(this.tokenKey);
        localStorage.removeItem(this.userKey);
        localStorage.removeItem(this.demoModeKey);
    }

    /**
     * Decode JWT token
     */
    decodeJWT(token) {
        const parts = token.split('.');
        if (parts.length !== 3) {
            throw new Error('Invalid JWT token format');
        }

        const payload = JSON.parse(this.base64UrlDecode(parts[1]));
        return payload;
    }

    /**
     * Base64 URL encode
     */
    base64UrlEncode(data) {
        if (typeof data === 'string') {
            data = new TextEncoder().encode(data);
        }
        
        const base64 = btoa(String.fromCharCode(...data));
        return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
    }

    /**
     * Base64 URL decode
     */
    base64UrlDecode(str) {
        str = str.replace(/-/g, '+').replace(/_/g, '/');
        while (str.length % 4) {
            str += '=';
        }
        return atob(str);
    }

    /**
     * Get authorization header for API requests
     */
    getAuthHeader() {
        // In demo mode, don't send Authorization header
        if (this.isDemoMode()) {
            return null;
        }
        
        const token = this.getToken();
        return token ? `Bearer ${token}` : null;
    }
}

// Create global instance
window.AuthManager = new AuthManager();
