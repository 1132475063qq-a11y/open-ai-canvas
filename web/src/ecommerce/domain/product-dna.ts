export type ProductFactValue = string | number | boolean | string[];

export type ProductFactMap = Record<string, ProductFactValue>;

export type ProductDNA = {
    category: string;
    color: string;
    shape: string;
    visibleMaterial: string[];
    logo: string;
    packaging: string;
    keyFeatures: string[];
    confirmedFacts: ProductFactMap;
    inferredFacts: ProductFactMap;
    unknownFacts: string[];
    forbiddenClaims: string[];
    mustPreserve: string[];
    allowedVariations: string[];
    forbiddenChanges: string[];
    confidence: number;
    sourceAssets: string[];
};

export type ProductDNAInput = Partial<ProductDNA> & Pick<ProductDNA, "sourceAssets">;

export function createProductDNA(input: ProductDNAInput): ProductDNA {
    const confidence = input.confidence ?? 0;
    if (confidence < 0 || confidence > 1) {
        throw new Error("ProductDNA confidence must be between 0 and 1");
    }
    if (input.sourceAssets.length === 0) {
        throw new Error("ProductDNA requires at least one source asset");
    }
    return {
        category: input.category ?? "",
        color: input.color ?? "",
        shape: input.shape ?? "",
        visibleMaterial: [...(input.visibleMaterial ?? [])],
        logo: input.logo ?? "",
        packaging: input.packaging ?? "",
        keyFeatures: [...(input.keyFeatures ?? [])],
        confirmedFacts: { ...(input.confirmedFacts ?? {}) },
        inferredFacts: { ...(input.inferredFacts ?? {}) },
        unknownFacts: [...(input.unknownFacts ?? [])],
        forbiddenClaims: [...(input.forbiddenClaims ?? [])],
        mustPreserve: [...(input.mustPreserve ?? [])],
        allowedVariations: [...(input.allowedVariations ?? [])],
        forbiddenChanges: [...(input.forbiddenChanges ?? [])],
        confidence,
        sourceAssets: [...new Set(input.sourceAssets)],
    };
}
