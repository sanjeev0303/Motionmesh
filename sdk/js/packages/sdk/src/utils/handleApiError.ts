export const handleApiError = async (
  response: Response,
  context: string,
): Promise<never> => {
  if (response.status === 404) {
    throw new Error(
      "Motionmesh API route not found. " +
        "Make sure you have created the file at " +
        "app/api/motionmesh/route.ts",
    );
  }

  // Try to extract the real error message
  let errorMessage = "Request failed";

  switch (context) {
    case "initiate":
      errorMessage = "Upload initiation failed";
      break;
    case "complete":
      errorMessage = "Upload completion failed";
      break;
    case "preview":
      errorMessage = "Failed to get the file preview";
      break;
    default:
      break;
  }

  try {
    const errorData = await response.json();

    if (errorData.error) {
      errorMessage = errorData.error;
    } else if (errorData.message) {
      errorMessage = errorData.message;
    }
  } catch (error) {
    errorMessage = response.statusText || errorMessage;
  }

  throw new Error(`[Motionmesh] ${errorMessage}`);
};
