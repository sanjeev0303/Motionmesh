export interface EnvConfig {
    baseUrl: string;
}

export const getEnvConfig = (): EnvConfig => {
    return {
        baseUrl: process.env.MOTIONMESH_BASE_URL || "http://localhost:8080/v1",
    };
};
