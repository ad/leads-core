/**
 * API Client for Leads Core Admin Panel
 * Handles all API communication with proper JWT authentication
 */

class APIClient {
    constructor() {
        this.baseURL = window.location.origin;
        this.token = null;
    }

    /**
     * Check if user is authenticated
     */
    isAuthenticated() {
        return window.AuthManager ? window.AuthManager.isAuthenticated() : false;
    }

    /**
     * Set authentication token
     */
    setToken(token) {
        this.token = token;
    }

    /**
     * Clear authentication
     */
    clearAuth() {
        this.token = null;
    }

    /**
     * Get authorization headers
     */
    getAuthHeaders() {
        const headers = {
            'Content-Type': 'application/json'
        };

        const authHeader = window.AuthManager ? window.AuthManager.getAuthHeader() : null;
        if (authHeader) {
            headers['Authorization'] = authHeader;
        }

        return headers;
    }

    /**
     * Check if in demo mode
     */
    isDemoMode() {
        return window.AuthManager && window.AuthManager.isDemoMode();
    }

    /**
     * Make HTTP request with proper error handling
     */
    async makeRequest(url, options = {}) {
        const headers = this.getAuthHeaders();
        
        const requestOptions = {
            ...options,
            headers: {
                ...headers,
                ...options.headers
            }
        };

        try {
            const response = await fetch(url, requestOptions);
            
            // Handle authentication errors
            if (response.status === 401) {
                if (window.AuthManager) {
                    window.AuthManager.logout();
                }
                this.clearAuth();
                throw new Error('Authentication failed. Please login again.');
            }

            // Handle other HTTP errors
            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
            }

            // For 204 No Content, return success status instead of trying to parse JSON
            if (response.status === 204) {
                return { success: true, status: 204 };
            }

            // Return JSON response for other successful responses
            return await response.json();
        } catch (error) {
            // Re-throw network errors or other issues
            if (error.name === 'TypeError' && error.message.includes('fetch')) {
                throw new Error('Network error. Please check your connection.');
            }
            throw error;
        }
    }

    /**
     * Test authentication with current token
     */
    async testAuth() {
        if (!this.isAuthenticated()) {
            throw new Error('No valid authentication token');
        }

        try {
            await this.getWidgetsSummary();
            return true;
        } catch (error) {
            throw new Error('Authentication test failed: ' + error.message);
        }
    }

    /**
     * Get widgets summary statistics
     */
    async getWidgetsSummary() {
        const url = `${this.baseURL}/api/v1/widgets/summary`;
        const response = await this.makeRequest(url);
        return response.data;
    }

    /**
     * Get current user information
     */
    async getCurrentUser() {
        const url = `${this.baseURL}/api/v1/user`;
        const response = await this.makeRequest(url);
        return response;
    }

    /**
     * Get widgets list with optional pagination and filtering
     */
    async getWidgets(options = {}) {
        const url = new URL(`${this.baseURL}/api/v1/widgets`);
        
        // Add query parameters
        Object.entries(options).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                url.searchParams.set(key, value);
            }
        });

        console.log('API Request URL:', url.toString()); // Debug log

        const response = await this.makeRequest(url.toString());
        console.log('API Response:', response); // Debug log
        
        return response;
    }

    /**
     * Get single widget by ID
     */
    async getWidget(widgetId) {
        const url = `${this.baseURL}/api/v1/widgets/${widgetId}`;
        const response = await this.makeRequest(url);
        return response;
    }

    /**
     * Get widget statistics
     */
    async getWidgetStats(widgetId) {
        const url = `${this.baseURL}/api/v1/widgets/${widgetId}/stats`;
        const response = await this.makeRequest(url);
        return response.data;
    }

    /**
     * Get widget submissions with optional pagination
     */
    async getWidgetSubmissions(widgetId, options = {}) {
        const url = new URL(`${this.baseURL}/api/v1/widgets/${widgetId}/submissions`);
        
        // Add query parameters
        Object.entries(options).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                url.searchParams.set(key, value);
            }
        });

        return await this.makeRequest(url.toString());
    }

    /**
     * Create a new widget
     */
    async createWidget(widgetData) {
        const url = `${this.baseURL}/api/v1/widgets`;
        return await this.makeRequest(url, {
            method: 'POST',
            body: JSON.stringify(widgetData)
        });
    }

    /**
     * Update an existing widget
     */
    async updateWidget(widgetId, widgetData) {
        const url = `${this.baseURL}/api/v1/widgets/${widgetId}`;
        return await this.makeRequest(url, {
            method: 'POST',
            body: JSON.stringify(widgetData)
        });
    }

    /**
     * Update widget configuration
     */
    async updateWidgetConfig(widgetId, configData) {
        const url = `${this.baseURL}/api/v1/widgets/${widgetId}/config`;
        return await this.makeRequest(url, {
            method: 'PUT',
            body: JSON.stringify(configData)
        });
    }

    /**
     * Delete a widget
     */
    async deleteWidget(widgetId) {
        const url = `${this.baseURL}/api/v1/widgets/${widgetId}`;
        return await this.makeRequest(url, {
            method: 'DELETE'
        });
    }

    /**
     * Health check
     */
    async healthCheck() {
        const url = `${this.baseURL}/health`;
        return await this.makeRequest(url);
    }

    /**
     * Send widget event (view, close)
     */
    async sendWidgetEvent(widgetId, eventType) {
        const url = `${this.baseURL}/widgets/${widgetId}/events`;
        return await this.makeRequest(url, {
            method: 'POST',
            body: JSON.stringify({ type: eventType })
        });
    }

    /**
     * Submit test widget data
     */
    async submitTestWidgetData(widgetId, testData) {
        const url = `${this.baseURL}/widgets/${widgetId}/submit`;
        return await this.makeRequest(url, {
            method: 'POST',
            body: JSON.stringify(testData)
        });
    }

    /**
     * Get recent widget submissions for preview
     */
    async getRecentSubmissions(widgetId, limit = 5) {
        const url = `${this.baseURL}/api/v1/widgets/${widgetId}/submissions?limit=${limit}&sort=desc`;
        return await this.makeRequest(url);
    }
}

// Create global instance
window.APIClient = new APIClient();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = APIClient;
}
