package com.amitia.amitia_app.runtime.proot.internal

import android.content.Context
import com.amitia.amitia_app.runtime.proot.ProotArtifact
import com.amitia.amitia_app.runtime.proot.ProotBinaryLocator
import java.io.File

internal class AndroidProotBinaryLocator(private val context: Context, private val metadataLoader: ProotMetadataLoaderInternal) : ProotBinaryLocator {
    override fun locate(): File? {
        val artifact = metadataLoader.load() ?: return null
        val nativeLibDir = context.applicationInfo.nativeLibraryDir ?: return null
        val file = File(nativeLibDir, artifact.fileName)
        return if (file.exists() && file.isFile) file else null
    }
}
internal interface ProotMetadataLoaderInternal { fun load(): ProotArtifact? }